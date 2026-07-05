package session

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrevent"
	"github.com/mondegor/go-core/mrlog"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/repository"
)

type (
	// ContinueSession - продолжение сессии: перевыпуск пары токенов по refresh токену.
	ContinueSession struct {
		storage        authTokenStorage
		tokenRecreator tokenRecreator
		eventEmitter   mrevent.Emitter
		errorWrapper   errors.Wrapper
		logger         mrlog.Logger
	}

	authTokenStorage interface {
		RevokeTokensBySessionID(ctx context.Context, userID uuid.UUID, sessionID uint32) error
	}

	tokenRecreator interface {
		Recreate(ctx context.Context, refreshToken string) (token dto.AuthTokenPair, err error)
	}
)

// NewContinueSession - создаёт объект ContinueSession.
func NewContinueSession(
	storage authTokenStorage,
	tokenRecreator tokenRecreator,
	eventEmitter mrevent.Emitter,
	logger mrlog.Logger,
) *ContinueSession {
	return &ContinueSession{
		storage:        storage,
		tokenRecreator: tokenRecreator,
		eventEmitter:   eventEmitter,
		errorWrapper:   errors.NewServiceRecordNotFoundWrapper(),
		logger:         logger,
	}
}

// Execute - перевыпускает пару токенов по refresh токену; при обнаружении переиспользования
// отозванного токена вне окна действия отзывает всю сессию.
func (uc *ContinueSession) Execute(ctx context.Context, _, refreshToken string) (authToken dto.AuthTokenPair, err error) {
	if refreshToken == "" {
		return dto.AuthTokenPair{}, errors.ErrIncorrectInputData.New("refreshToken is empty")
	}

	authToken, err = uc.tokenRecreator.Recreate(ctx, refreshToken)
	if err != nil {
		var tokenErr *repository.TokenAlreadyRevokedError

		if errors.As(err, &tokenErr) {
			// повторное использование отозванного refresh токена вне окна его действия
			if err := uc.storage.RevokeTokensBySessionID(ctx, tokenErr.UserID, tokenErr.SessionID); err != nil {
				uc.logger.Error(ctx, "RevokeAlert.RevokeTokensBySessionID", "error", err)
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

			uc.eventEmitter.Emit(ctx, "RevokeAlert", "userId", tokenErr.UserID)

			return dto.AuthTokenPair{}, mrauth.ErrTokenNotFoundOrExpired
		}

		if errors.Is(err, errors.ErrEventStorageNoRecordFound) || errors.Is(err, repository.ErrTokenExpired) {
			return dto.AuthTokenPair{}, mrauth.ErrTokenNotFoundOrExpired
		}

		return dto.AuthTokenPair{}, uc.errorWrapper.Wrap(err)
	}

	return authToken, nil
}
