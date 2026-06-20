package session

import (
	"context"
	"crypto/rand"
	"math"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrlock"
	"github.com/mondegor/go-sysmess/mrstorage"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
)

const (
	openSessionLockKeyPrefix = "auth.open-session:"
	openSessionLockTimeout   = 15 * time.Second
	defaultSessionMax        = 4 // максимум одновременных сессий, если для kind не задан лимит
)

type (
	// OpenSession - открытие новой сессии после подтверждённой операции авторизации.
	OpenSession struct {
		txManager             mrstorage.DBTxManager
		storageSession        sessionStorage
		storageUserActivity   userActivityStatCreator
		openSessionFetcher    openSessionFetcher
		sessionCloser         sessionCloser
		locker                mrlock.Locker // ограничивает частоту открытия сессий пользователя (кулдаун входа)
		handlerCreateUser     operationHandlerCreateUser
		handlerBeforeAuthUser operationHandlerBeforeAuthUser
		tokenCreator          tokenCreator
		sessionLimits         map[realmKindKey]int
		errorWrapper          errors.Wrapper
	}

	// realmKindKey - составной ключ "realm + вид пользователя" для мапы лимитов сессий.
	realmKindKey struct {
		realm string
		kind  string
	}

	// LimitRealm - лимиты одновременных сессий по видам пользователя (kind) внутри realm.
	LimitRealm struct {
		Name       string
		KindLimits []UserKindLimit
	}

	// UserKindLimit - лимит одновременных сессий для вида пользователя (kind).
	UserKindLimit struct {
		Kind       string
		SessionMax uint32
	}

	sessionStorage interface {
		Insert(ctx context.Context, row entity.Session) error
	}

	userActivityStatCreator interface {
		InsertOrUpdate(ctx context.Context, row entity.UserActivityStat) error
	}

	operationHandlerCreateUser interface {
		Execute(ctx context.Context, payload []byte) (userScopes dto.UserScopes, err error) // сделать DTO и объединить CreateUser + BeforeAuthUser интерфейсы
	}

	operationHandlerBeforeAuthUser interface {
		Execute(ctx context.Context, userID uuid.UUID, payload []byte) (userScopes dto.UserScopes, err error) // сделать DTO
	}

	tokenCreator interface {
		Create(ctx context.Context, userScopes dto.UserScopes) (token dto.AuthTokenPair, err error)
	}
)

// NewOpenSession - создаёт объект OpenSession.
func NewOpenSession(
	txManager mrstorage.DBTxManager,
	storageSession sessionStorage,
	storageUserActivity userActivityStatCreator,
	openSessionFetcher openSessionFetcher,
	sessionCloser sessionCloser,
	locker mrlock.Locker,
	handlerCreateUser operationHandlerCreateUser,
	handlerBeforeAuthUser operationHandlerBeforeAuthUser,
	tokenCreator tokenCreator,
	allowedRealms []LimitRealm,
) *OpenSession {
	sessionLimits := make(map[realmKindKey]int)

	for _, realm := range allowedRealms {
		for _, kind := range realm.KindLimits {
			// не заданный лимит (0) заменяется значением по умолчанию
			if kind.SessionMax < 1 {
				kind.SessionMax = defaultSessionMax
			}

			sessionLimits[realmKindKey{realm: realm.Name, kind: kind.Kind}] = int(kind.SessionMax)
		}
	}

	return &OpenSession{
		txManager:             txManager,
		storageSession:        storageSession,
		storageUserActivity:   storageUserActivity,
		openSessionFetcher:    openSessionFetcher,
		sessionCloser:         sessionCloser,
		locker:                locker,
		handlerCreateUser:     handlerCreateUser,
		handlerBeforeAuthUser: handlerBeforeAuthUser,
		tokenCreator:          tokenCreator,
		sessionLimits:         sessionLimits,
		errorWrapper:          errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - открывает новую сессию: генерирует её идентификатор, выпускает пару токенов,
// сохраняет строку сессии и фиксирует активность пользователя.
func (uc *OpenSession) Execute(ctx context.Context, meta dto.SessionMeta, op secureoperation.SecureOperation) (authToken dto.AuthTokenPair, err error) {
	var userScopes dto.UserScopes

	if !op.Is(operationstatus.Confirmed) {
		return dto.AuthTokenPair{}, secureoperation.ErrOperationIsNotConfirmed
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		switch op.Name {
		case unit.NameConfirmCreateUser:
			userScopes, err = uc.handlerCreateUser.Execute(ctx, op.Payload)
			if err != nil {
				return err
			}
		case unit.NameAuthorizeUser:
			userScopes, err = uc.handlerBeforeAuthUser.Execute(ctx, op.UserID, op.Payload)
			if err != nil {
				return err
			}
		default:
			return errors.ErrAccessForbidden
		}

		sessionID, err := genSessionID()
		if err != nil {
			return err
		}

		userScopes.SessionID = sessionID

		// кулдаун входа по пользователю: не более одного открытия сессии за openSessionLockTimeout.
		// Берётся до подсчёта сессий и удерживается через выпуск токенов и commit, поэтому закрывает
		// гонку (TOCTOU), когда два конкурентных логина оба прошли бы проверку лимита и вместе его превысили.
		openSessionUnlock, err := uc.locker.LockWithExpiry(ctx, openSessionLockKeyPrefix+userScopes.UserID.String(), openSessionLockTimeout)
		if err != nil {
			if errors.Is(err, mrlock.ErrLockKeyNotObtained) {
				return mrauth.ErrTooManyOpenSessionRequests
			}

			return err
		}
		// при ошибке снимаем блокировку сразу - неудачный логин не должен наказываться кулдауном;
		// при успехе не трогаем: она сама истечёт по таймауту и работает как кулдаун до следующего входа
		defer func() {
			if err != nil {
				openSessionUnlock()
			}
		}()

		// проверка лимита до выпуска токенов: Create вставляет refresh-токен новой сессии,
		// поэтому подсчёт открытых сессий должен пройти раньше, иначе новая сессия посчитает сама себя
		if err = uc.enforceSessionLimit(ctx, userScopes.UserID, userScopes.Realm, userScopes.Kind); err != nil {
			return err
		}

		authToken, err = uc.tokenCreator.Create(ctx, userScopes)
		if err != nil {
			return err
		}

		// realIP=0 при ошибке/IPv6 - поток login не должен падать из-за этого
		realIP, _, _ := meta.ClientIP.ToUint()

		if err = uc.storageSession.Insert(ctx, entity.Session{
			UserID:    userScopes.UserID,
			SessionID: sessionID,
			UserAgent: meta.UserAgent,
			LastIP:    realIP,
		}); err != nil {
			return err
		}

		userActivity := entity.UserActivityStat{
			UserID:        userScopes.UserID,
			LastLoginIP:   meta.ClientIP,
			LastLoggedAt:  time.Now(),
			LastVisitedAt: time.Now(),
		}

		// TODO: возможно здесь можно вставлять эту запись и асинхронно
		// результат присваиваем err (а не делаем bare return), иначе defer выше не увидит ошибку
		// этого шага и не снимет блокировку
		err = uc.storageUserActivity.InsertOrUpdate(ctx, userActivity)

		return err
	})
	if err != nil {
		return dto.AuthTokenPair{}, uc.errorWrapper.Wrap(err)
	}

	return authToken, nil
}

// enforceSessionLimit - если открытие новой сессии превысит лимит вида пользователя,
// закрывает наименее активные открытые сессии, чтобы освободить место.
// FetchOpenSessionIDs возвращает сессии, отсортированные по возрасту refresh токена
// (наименее активные первыми), поэтому закрывается префикс списка.
func (uc *OpenSession) enforceSessionLimit(ctx context.Context, userID uuid.UUID, realm, kind string) error {
	// limit == 0 означает, что realm/kind не сконфигурирован - применяется значение по умолчанию
	limit := uc.sessionLimits[realmKindKey{realm: realm, kind: kind}]
	if limit == 0 {
		limit = defaultSessionMax
	}

	openIDs, err := uc.openSessionFetcher.FetchOpenSessionIDs(ctx, userID)
	if err != nil {
		return err
	}

	// +1 - место под открываемую сессию
	toClose := len(openIDs) + 1 - limit
	if toClose <= 0 {
		return nil
	}

	return uc.sessionCloser.RevokeTokensBySessionIDs(ctx, userID, openIDs[:toClose])
}

// TODO: временно, потом переделать через интерфейс.
func genSessionID() (uint32, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(math.MaxUint32))
	if err != nil {
		return 0, err
	}

	// n принадлежит [0, math.MaxUint32), результат [1, math.MaxUint32] помещается в uint32
	return uint32(n.Uint64()) + 1, nil //nolint:gosec
}
