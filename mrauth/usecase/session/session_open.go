package session

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrtype"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	secureoperation2 "github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
)

type (
	// OpenSession - comment struct.
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
		Create(ctx context.Context, userScopes dto.UserScopes) (token dto.AuthToken, err error)
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

// Execute - comments method.
func (uc *OpenSession) Execute(ctx context.Context, clientIP mrtype.DetailedIP, op secureoperation2.SecureOperation) (authToken dto.AuthToken, err error) {
	var userScopes dto.UserScopes

	if !op.Is(operationstatus.Confirmed) {
		return dto.AuthToken{}, secureoperation2.ErrOperationIsNotConfirmed
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
		return dto.AuthToken{}, uc.errorWrapper.Wrap(err)
	}

	return authToken, nil
}
