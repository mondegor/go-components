package session

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrevent"
	"github.com/mondegor/go-core/mrlog"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/enum/securityevent"
)

const (
	// sourceNameContinue - источник событий продления сессии для журнала. В этом потоке защищённой
	// операции нет вообще (перевыпуск идёт по refresh-токену), поэтому источник задаётся константой.
	sourceNameContinue = "SESSION_CONTINUE"
)

type (
	// ContinueSession - продолжение сессии: перевыпуск пары токенов по refresh токену.
	ContinueSession struct {
		storage        authTokenStorage
		tokenRecreator tokenRecreator
		eventEmitter   mrevent.Emitter
		logOperation   operationLogger
		securityLog    securityLogStorage
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
	logOperation operationLogger,
	securityLog securityLogStorage,
	logger mrlog.Logger,
) *ContinueSession {
	return &ContinueSession{
		storage:        storage,
		tokenRecreator: tokenRecreator,
		eventEmitter:   eventEmitter,
		logOperation:   logOperation,
		securityLog:    securityLog,
		errorWrapper:   errors.NewServiceOperationFailedWrapper(),
		logger:         logger,
	}
}

// Execute - перевыпускает пару токенов по refresh токену; при обнаружении переиспользования
// отозванного токена вне окна действия отзывает всю сессию и записывает событие в журнал
// безопасности пользователя.
func (uc *ContinueSession) Execute(ctx context.Context, actor dto.ActorMeta, _, refreshToken string) (authToken dto.AuthTokenPair, err error) {
	if refreshToken == "" {
		return dto.AuthTokenPair{}, mrauth.ErrTokenNotFoundOrExpired
	}

	authToken, err = uc.tokenRecreator.Recreate(ctx, refreshToken)
	if err != nil {
		var tokenErr *mrauth.TokenAlreadyRevokedError

		if errors.As(err, &tokenErr) {
			uc.handleTokenReuse(ctx, actor, tokenErr)

			return dto.AuthTokenPair{}, mrauth.ErrTokenNotFoundOrExpired
		}

		if errors.Is(err, errors.ErrEventStorageNoRecordFound) || errors.Is(err, mrauth.ErrEventTokenExpired) {
			return dto.AuthTokenPair{}, mrauth.ErrTokenNotFoundOrExpired
		}

		return dto.AuthTokenPair{}, uc.errorWrapper.Wrap(err)
	}

	return authToken, nil
}

// handleTokenReuse - реагирует на повторное использование отозванного refresh токена вне окна
// его действия (атака): отзывает сессию, фиксирует блокировку в журнале защищённых операций,
// а если сессия была открыта - событие в журнале безопасности пользователя. Сбои отдельных
// шагов только логируются: клиенту в любом случае отдаётся отказ по токену.
func (uc *ContinueSession) handleTokenReuse(ctx context.Context, actor dto.ActorMeta, tokenErr *mrauth.TokenAlreadyRevokedError) {
	// сессия, в которой повторён токен, считается скомпрометированной
	sessionWasOpen := true

	if err := uc.storage.RevokeTokensBySessionID(ctx, tokenErr.UserID, tokenErr.SessionID); err != nil {
		if errors.Is(err, errors.ErrEventStorageRecordsNotAffected) {
			// сессию уже закрыли (выход, закрытие из списка, предыдущий повтор) или она истекла: это не сбой
			sessionWasOpen = false
		} else {
			uc.logger.Error(ctx, "RevokeAlert.RevokeTokensBySessionID", "error", err)
		}
	}

	// повторное использование refresh-токена (атака): фиксируем блокировку в журнале
	uc.logOperation.Log(
		ctx,
		actor.WithUser(tokenErr.UserID).NewOperationLog(
			sourceNameContinue, confirmmethod.Unspecified, logstatus.Blocked, logreason.TokenReuse,
		),
	)

	// повтор токена уже закрытой сессии ничего не меняет - это неудачная попытка, такие
	// в журнал безопасности не пишутся; при сбое отзыва состояние сессии неизвестно, и событие
	// пишется, чтобы не потерять предупреждение
	if sessionWasOpen {
		// транзакции здесь нет, поэтому сбой записи только логируется
		if err := uc.securityLog.Insert(
			ctx,
			actor.WithUser(tokenErr.UserID).NewSecurityEvent(securityevent.TokenReuseDetected, nil),
		); err != nil {
			uc.logger.Error(ctx, "RevokeAlert.SecurityLog", "error", err)
		}
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
}
