package auth

import (
	"context"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// operationNamer - общее у фабрик обоих маршрутов входа: имя создаваемой операции,
	// нужное журналу ещё до её создания (pre-op).
	operationNamer interface {
		Name() string
	}

	// createSessionFlow - общий конвейер создания операции входа для юзкейсов, различающихся
	// только цепочкой подтверждения: обычный вход (CreateSession) и вход пользователя,
	// утратившего доступ к почте (CreateSessionByRecovery).
	createSessionFlow[T operationNamer] struct {
		opener                      operationOpener
		userChecker                 userLoginChecker
		factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator
		logOperation                operationLogger
		errorWrapper                errors.Wrapper
		realm2operation             map[string]T
		noteName                    string
	}
)

// newCreateSessionFlow - создаёт объект createSessionFlow.
// noteName - имя шаблона уведомления с кодом подтверждения; пустое значение означает,
// что первое звено цепочки не sendable и отправлять нечего.
func newCreateSessionFlow[T operationNamer](
	opener operationOpener,
	userChecker userLoginChecker,
	factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator,
	logOperation operationLogger,
	realm2operation map[string]T,
	noteName string,
) createSessionFlow[T] {
	return createSessionFlow[T]{
		opener:                      opener,
		userChecker:                 userChecker,
		errorWrapper:                errors.NewServiceOperationFailedWrapper(),
		factoryUser2FAConfirmAction: factoryUser2FAConfirmAction,
		logOperation:                logOperation,
		realm2operation:             realm2operation,
		noteName:                    noteName,
	}
}

// execute - проверяет логин пользователя в рамках realm, создаёт операцию входа выбранной
// цепочкой (createFunc) и в той же транзакции открывает её.
func (f createSessionFlow[T]) execute(
	ctx context.Context,
	actor dto.ActorMeta,
	realm, langCode string,
	userLogin contactaddress.ContactAddress,
	createFunc func(opCreator T, user2FA dto.User2FA) (secureoperation.SecureOperation, error),
) (secureoperation.SecureOperation, error) {
	if langCode == "" {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("langCode is empty")
	}

	if userLogin.Value() == "" {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("userLogin is empty")
	}

	opCreator, ok := f.realm2operation[realm]
	if !ok {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("realm is unknown", "realm", realm)
	}

	err := f.userChecker.CheckAvailabilityRealm(ctx, realm, userLogin)
	if err == nil {
		// логина не существует: фиксируем в журнале попытку входа по несуществующему логину
		// (операция не создана, поэтому её имя берётся у фабрики, а метод подтверждения неизвестен)
		f.logOperation.Log(
			ctx,
			actor.NewOperationLog(
				opCreator.Name(), confirmmethod.Unspecified, logstatus.Blocked, logreason.LoginNotExists,
			),
		)

		return secureoperation.SecureOperation{}, mrauth.ErrLoginNotExists
	}

	if !errors.Is(err, mrauth.ErrEmailAlreadyExists) && !errors.Is(err, mrauth.ErrPhoneAlreadyExists) {
		return secureoperation.SecureOperation{}, f.errorWrapper.Wrap(err)
	}

	user2FA, err := f.factoryUser2FAConfirmAction.CreateByUserLogin(ctx, userLogin)
	if err != nil {
		return secureoperation.SecureOperation{}, f.errorWrapper.Wrap(err)
	}

	op, err := createFunc(opCreator, user2FA)
	if err != nil {
		return secureoperation.SecureOperation{}, f.errorWrapper.Wrap(err)
	}

	err = f.opener.Open(ctx, actor, op, f.noteName, conv.Group{"lang": langCode})
	if err != nil {
		return secureoperation.SecureOperation{}, f.errorWrapper.Wrap(err)
	}

	return op, nil
}
