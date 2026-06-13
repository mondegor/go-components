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
	"github.com/mondegor/go-sysmess/mrtype"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
)

//go:generate mockgen -source=session_open.go -destination=mock/session_open.go -package=mock
//go:generate mockgen -source=session_continue.go -destination=mock/session_continue.go -package=mock
//go:generate mockgen -source=session_close.go -destination=mock/session_close.go -package=mock
//go:generate mockgen -destination=mock/mrstorage.go -package=mock github.com/mondegor/go-sysmess/mrstorage DBTxManager
//go:generate mockgen -destination=mock/mrevent.go -package=mock github.com/mondegor/go-sysmess/mrevent Emitter

type (
	// OpenSession - открытие новой сессии после подтверждённой операции авторизации.
	OpenSession struct {
		txManager             mrstorage.DBTxManager
		storageUserActivity   userActivityStatCreator
		handlerCreateUser     operationHandlerCreateUser
		handlerBeforeAuthUser operationHandlerBeforeAuthUser
		tokenCreator          tokenCreator
		errorWrapper          errors.Wrapper
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
	storageUserActivity userActivityStatCreator,
	handlerCreateUser operationHandlerCreateUser,
	handlerBeforeAuthUser operationHandlerBeforeAuthUser,
	tokenCreator tokenCreator,
) *OpenSession {
	return &OpenSession{
		txManager:             txManager,
		storageUserActivity:   storageUserActivity,
		handlerCreateUser:     handlerCreateUser,
		handlerBeforeAuthUser: handlerBeforeAuthUser,
		tokenCreator:          tokenCreator,
		errorWrapper:          errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - открывает новую сессию: генерирует её идентификатор, выпускает пару токенов и фиксирует активность пользователя.
func (uc *OpenSession) Execute(ctx context.Context, clientIP mrtype.DetailedIP, op secureoperation.SecureOperation) (authToken dto.AuthTokenPair, err error) {
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

		userActivity := entity.UserActivityStat{
			UserID:        userScopes.UserID,
			LastLoginIP:   clientIP,
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
