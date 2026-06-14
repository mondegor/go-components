package session

import (
	"context"
	"crypto/rand"
	"math"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
)

type (
	// OpenSession - открытие новой сессии после подтверждённой операции авторизации.
	OpenSession struct {
		txManager             mrstorage.DBTxManager
		storageSession        sessionStorage
		storageUserActivity   userActivityStatCreator
		handlerCreateUser     operationHandlerCreateUser
		handlerBeforeAuthUser operationHandlerBeforeAuthUser
		tokenCreator          tokenCreator
		errorWrapper          errors.Wrapper
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
	handlerCreateUser operationHandlerCreateUser,
	handlerBeforeAuthUser operationHandlerBeforeAuthUser,
	tokenCreator tokenCreator,
) *OpenSession {
	return &OpenSession{
		txManager:             txManager,
		storageSession:        storageSession,
		storageUserActivity:   storageUserActivity,
		handlerCreateUser:     handlerCreateUser,
		handlerBeforeAuthUser: handlerBeforeAuthUser,
		tokenCreator:          tokenCreator,
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

		return uc.storageUserActivity.InsertOrUpdate(ctx, userActivity)
	})
	if err != nil {
		return dto.AuthTokenPair{}, uc.errorWrapper.Wrap(err)
	}

	return authToken, nil
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
