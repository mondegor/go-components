package session

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrargs"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"
	"github.com/mondegor/go-sysmess/mrevent"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrtype"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/component/secureoperation"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/repository"
)

type (
	// Session - comment struct.
	Session struct {
		txManager             mrstorage.DBTxManager
		storage               mrauth.AuthTokenStorage
		storageUserActivity   mrauth.UserActivityStatStorage
		storageOperation      mrauth.SecureOperationStorage
		handlerCreateUser     operationHandlerCreateUser
		handlerBeforeAuthUser operationHandlerBeforeAuthUser
		eventEmitter          mrevent.Emitter
		errorWrapper          mrerr.UseCaseErrorWrapper
		logger                mrlog.Logger
		realm2tokenIssuer     map[string]mrauth.TokenIssuer
	}

	// CreateSessionRealm - сообщение для получателя.
	CreateSessionRealm struct {
		Name        string
		TokenIssuer mrauth.TokenIssuer
	}

	operationHandlerCreateUser interface {
		Execute(ctx context.Context, payload []byte) (user dto.UserInRealm, err error) // сделать DTO и объединить CreateUser + BeforeAuthUser интерфейсы
	}

	operationHandlerBeforeAuthUser interface {
		Execute(ctx context.Context, userID uuid.UUID, payload []byte) (user dto.UserInRealm, err error) // сделать DTO
	}
)

// NewSession - создаёт объект Session.
func NewSession(
	txManager mrstorage.DBTxManager,
	storage mrauth.AuthTokenStorage,
	storageUserActivity mrauth.UserActivityStatStorage,
	storageOperation mrauth.SecureOperationStorage,
	handlerCreateUser operationHandlerCreateUser,
	handlerBeforeAuthUser operationHandlerBeforeAuthUser,
	eventEmitter mrevent.Emitter,
	errorWrapper mrerr.UseCaseErrorWrapper,
	logger mrlog.Logger,
	allowedRealms []CreateSessionRealm,
) *Session {
	realm2tokenIssuer := make(map[string]mrauth.TokenIssuer, len(allowedRealms))
	for _, realm := range allowedRealms {
		realm2tokenIssuer[realm.Name] = realm.TokenIssuer
	}

	return &Session{
		txManager:             txManager,
		storage:               storage,
		storageUserActivity:   storageUserActivity,
		storageOperation:      storageOperation,
		eventEmitter:          eventEmitter,
		handlerCreateUser:     handlerCreateUser,
		handlerBeforeAuthUser: handlerBeforeAuthUser,
		errorWrapper:          mrerr.NewUseCaseErrorWrapper(errorWrapper, entity.ModelNameRefreshToken),
		logger:                logger,
		realm2tokenIssuer:     realm2tokenIssuer,
	}
}

// Open - comments method.
func (uc *Session) Open(ctx context.Context, clientIP mrtype.DetailedIP, op entity.SecureOperation) (authToken dto.AuthToken, err error) {
	var user dto.UserInRealm

	if op.Status != operationstatus.Confirmed {
		return dto.AuthToken{}, mrauth.ErrOperationIsNotConfirmed.New()
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
			return mr.ErrUseCaseAccessForbidden.New()
		}

		authToken, err = uc.createAuthToken(ctx, user.Realm, user.Kind, user.LangCode, user.ID)
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
		return dto.AuthToken{}, err
	}

	return authToken, nil
}

// Continue - возвращает строковое значение настройки с указанным идентификатором.
func (uc *Session) Continue(ctx context.Context, _, refreshToken string) (authToken dto.AuthToken, err error) {
	if refreshToken == "" {
		return dto.AuthToken{}, mr.ErrUseCaseIncorrectInputData.New("refreshToken is empty")
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		userScopes, err := uc.storage.Revoke(ctx, refreshToken)
		if err != nil {
			if repository.ErrTokenAlreadyRevoked.Is(err) {
				if err := uc.storage.UpdateToCloseAll(ctx, userScopes.UserID); err != nil {
					uc.logger.Error(ctx, "RevokeAlert.UpdateToCloseAll", "error", err)
				}

				// TODO: отправлять предупреждение пользователю

				// err := uc.notifierAPI.SendNotice(
				//	 ctx,
				//	 "user.revoke.token.alert",
				//	 mrargs.Group{
				//		 "langCode": langCode,
				//		 "to": rights.UserID,
				//	 },
				// )
				// if err != nil {
				// 	 uc.logger.Error(ctx, "Notice 'user.revoke.token.alert' not send", "error", err)
				// }

				uc.eventEmitter.Emit(ctx, "RevokeAlert", mrargs.Group{"userId": userScopes.UserID})

				// возвращаемая ошибка специально обобщается
				return mrauth.ErrTokenNotFoundOrExpired.Wrap(mr.ErrUseCaseEntityNotFound)
			}

			if uc.errorWrapper.IsNotFoundOrNotAffectedError(err) || repository.ErrTokenExpired.Is(err) {
				// возвращаемая ошибка специально обобщается
				return mrauth.ErrTokenNotFoundOrExpired.Wrap(mr.ErrUseCaseEntityNotFound)
			}

			return uc.errorWrapper.WrapErrorFailed(err)
		}

		authToken, err = uc.createAuthToken(ctx, userScopes.Realm, userScopes.UserKind, userScopes.LangCode, userScopes.UserID)

		return err
	})
	if err != nil {
		return dto.AuthToken{}, err
	}

	return authToken, nil
}

// Close - comments method.
func (uc *Session) Close(ctx context.Context, accessToken string) error {
	if accessToken == "" {
		return mr.ErrUseCaseIncorrectInputData.New("accessToken is empty")
	}

	// :TODO можно закрывать сессию по refresh token при jwt, иначе сейчас генерируется ошибка 404

	if err := uc.storage.UpdateToClose(ctx, accessToken); err != nil {
		if uc.errorWrapper.IsNotFoundOrNotAffectedError(err) {
			return mrauth.ErrTokenNotFoundOrExpired.Wrap(mr.ErrUseCaseEntityNotFound)
		}

		return uc.errorWrapper.WrapErrorFailed(err)
	}

	return nil
}

func (uc *Session) createAuthToken(ctx context.Context, realm, userKind, langCode string, userID uuid.UUID) (token dto.AuthToken, err error) {
	tokenIssuer, ok := uc.realm2tokenIssuer[realm]
	if !ok {
		return dto.AuthToken{}, mr.ErrUseCaseIncorrectInputData.New("realm is unknown", "realm", realm)
	}

	token, err = tokenIssuer.Create(realm, userKind, langCode, userID)
	if err != nil {
		return dto.AuthToken{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	authToken := entity.AuthToken{
		AccessToken:     token.AccessToken,
		RefreshToken:    token.RefreshToken,
		AccessExpiresAt: time.Now().Add(token.ExpiresIn).Round(1 * time.Second),
		Scopes: dto.AuthTokenScopes{
			Realm:    token.Scopes.Realm,
			UserKind: token.Scopes.UserKind,
			LangCode: token.Scopes.LangCode,
			UserID:   token.Scopes.UserID,
		},
		ExpiresAt: time.Now().Add(token.RefreshExpiresIn).Round(1 * time.Second),
	}

	// accessToken сохраняется в БД только у сессионных токенов,
	// подписанные токены типа jwt распаковываются без обращения к БД
	if token.HasSignature {
		authToken.AccessToken = ""
	}

	if err = uc.storage.Insert(ctx, authToken); err != nil {
		return dto.AuthToken{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	return token, nil
}
