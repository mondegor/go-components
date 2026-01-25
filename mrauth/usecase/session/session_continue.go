package session

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrevent"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/repository"
)

type (
	// ContinueSession - comment struct.
	ContinueSession struct {
		txManager      mrstorage.DBTxManager
		storage        mrauth.AuthTokenStorage
		tokenRecreator tokenRecreator
		eventEmitter   mrevent.Emitter
		errorWrapper   errors.Wrapper
		logger         mrlog.Logger
	}

	tokenRecreator interface {
		Create(ctx context.Context, realm, userKind, langCode string, userID uuid.UUID) (token dto.AuthToken, err error)
		Revoke(ctx context.Context, refreshToken string) (row dto.AuthTokenScopes, err error)
	}
)

// NewContinueSession - создаёт объект ContinueSession.
func NewContinueSession(
	txManager mrstorage.DBTxManager,
	storage mrauth.AuthTokenStorage,
	tokenRecreator tokenRecreator,
	eventEmitter mrevent.Emitter,
	logger mrlog.Logger,
) *ContinueSession {
	return &ContinueSession{
		txManager:      txManager,
		storage:        storage,
		tokenRecreator: tokenRecreator,
		eventEmitter:   eventEmitter,
		errorWrapper:   errors.NewUseCaseWrapper(),
		logger:         logger,
	}
}

// Execute - возвращает строковое значение настройки с указанным идентификатором.
func (uc *ContinueSession) Execute(ctx context.Context, _, refreshToken string) (authToken dto.AuthToken, err error) {
	if refreshToken == "" {
		return dto.AuthToken{}, errors.ErrUseCaseIncorrectInputData.New("refreshToken is empty")
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		userScopes, err := uc.storage.Revoke(ctx, refreshToken)
		if err != nil {
			if errors.Is(err, repository.ErrTokenAlreadyRevoked) {
				if err := uc.storage.UpdateToCloseAll(ctx, userScopes.UserID); err != nil {
					uc.logger.Error(ctx, "RevokeAlert.UpdateToCloseAll", "error", err)
				}

				// TODO: отправлять предупреждение пользователю

				// err := uc.notifierAPI.Send(
				//	 ctx,
				//	 "user.revoke.token.alert",
				//	 conv.Group{
				//		 "langCode": langCode,
				//		 "to": rights.UserID,
				//	 },
				// )
				// if err != nil {
				// 	 uc.logger.Error(ctx, "Notice 'user.revoke.token.alert' not send", "error", err)
				// }

				uc.eventEmitter.Emit(ctx, "RevokeAlert", conv.Group{"userId": userScopes.UserID})

				return mrauth.ErrTokenNotFoundOrExpired
			}

			if errors.Is(err, errors.ErrEventStorageNoRowFound) || errors.Is(err, repository.ErrTokenExpired) {
				return mrauth.ErrTokenNotFoundOrExpired
			}

			return uc.errorWrapper.Wrap(err)
		}

		authToken, err = uc.tokenRecreator.Create(ctx, userScopes.Realm, userScopes.UserKind, userScopes.LangCode, userScopes.UserID)

		return err
	})
	if err != nil {
		return dto.AuthToken{}, err
	}

	return authToken, nil
}
