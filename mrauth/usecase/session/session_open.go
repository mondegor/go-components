package session

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrtype"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/component/secureoperation"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
)

type (
	// OpenSession - comment struct.
	OpenSession struct {
		txManager             mrstorage.DBTxManager
		storageUserActivity   mrauth.UserActivityStatStorage
		handlerCreateUser     operationHandlerCreateUser
		handlerBeforeAuthUser operationHandlerBeforeAuthUser
		tokenCreator          tokenCreator
		errorWrapper          errors.Wrapper
	}

	operationHandlerCreateUser interface {
		Execute(ctx context.Context, payload []byte) (user dto.UserInRealm, err error) // сделать DTO и объединить CreateUser + BeforeAuthUser интерфейсы
	}

	operationHandlerBeforeAuthUser interface {
		Execute(ctx context.Context, userID uuid.UUID, payload []byte) (user dto.UserInRealm, err error) // сделать DTO
	}

	tokenCreator interface {
		Create(ctx context.Context, realm, userKind, langCode string, userID uuid.UUID) (token dto.AuthToken, err error)
	}
)

// NewOpenSession - создаёт объект OpenSession.
func NewOpenSession(
	txManager mrstorage.DBTxManager,
	storageUserActivity mrauth.UserActivityStatStorage,
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
		errorWrapper:          errors.NewUseCaseWrapper(),
	}
}

// Execute - comments method.
func (uc *OpenSession) Execute(ctx context.Context, clientIP mrtype.DetailedIP, op entity.SecureOperation) (authToken dto.AuthToken, err error) {
	var user dto.UserInRealm

	if op.Status != operationstatus.Confirmed {
		return dto.AuthToken{}, mrauth.ErrOperationIsNotConfirmed
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		switch op.Name {
		case secureoperation.NameConfirmCreateUser:
			user, err = uc.handlerCreateUser.Execute(ctx, op.Payload)
			if err != nil {
				return err
			}
		case secureoperation.NameAuthorizeUser:
			user, err = uc.handlerBeforeAuthUser.Execute(ctx, op.UserID, op.Payload)
			if err != nil {
				return err
			}
		default:
			return errors.ErrUseCaseAccessForbidden
		}

		authToken, err = uc.tokenCreator.Create(ctx, user.Realm, user.Kind, user.LangCode, user.ID)
		if err != nil {
			return err
		}

		userActivity := entity.UserActivityStat{
			UserID:        user.ID,
			LastLoginIP:   clientIP,
			LastLoggedAt:  time.Now(),
			LastVisitedAt: time.Now(),
		}

		return uc.storageUserActivity.InsertOrUpdate(ctx, userActivity)
	})
	if err != nil {
		return dto.AuthToken{}, uc.errorWrapper.Wrap(err)
	}

	return authToken, nil
}
