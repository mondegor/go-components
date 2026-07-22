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
	// CreateSession - инициирует создание сессии пользователя: подбирает операцию по
	// realm, создаёт её и отправляет код подтверждения по логину пользователя.
	CreateSession struct {
		opener                      operationOpener
		userChecker                 userLoginChecker
		factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator
		logOperation                operationLogger
		errorWrapper                errors.Wrapper
		realm2operation             map[string]createSessionOperation
	}

	// CreateSessionRealm - сопоставление realm с операцией создания сессии для него.
	CreateSessionRealm struct {
		Name      string
		Operation createSessionOperation
	}

	// operationOpener - открывает созданную операцию: гасит прежние операции того же
	// типа, сохраняет новую, отправляет код подтверждения и пишет журнал.
	operationOpener interface {
		Open(
			ctx context.Context,
			actor dto.ActorMeta,
			op secureoperation.SecureOperation,
			noteName string,
			noteProps conv.Group,
		) error
	}

	userLoginChecker interface {
		CheckAvailabilityRealm(ctx context.Context, realm string, userLogin contactaddress.ContactAddress) error
	}

	createSessionOperation interface {
		// Name - имя создаваемой операции; используется для событий журнала, возникающих
		// до её создания (pre-op), чтобы они не разъезжались с именем самой операции.
		Name() string
		Create(user2FA dto.User2FA, realm, langCode string, address contactaddress.ContactAddress) (secureoperation.SecureOperation, error)
	}
)

// NewCreateSession - создаёт объект CreateSession.
func NewCreateSession(
	opener operationOpener,
	userChecker userLoginChecker,
	factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator,
	logOperation operationLogger,
	allowedRealms []CreateSessionRealm,
) *CreateSession {
	realm2operation := make(map[string]createSessionOperation, len(allowedRealms))
	for _, realm := range allowedRealms {
		realm2operation[realm.Name] = realm.Operation
	}

	return &CreateSession{
		opener:                      opener,
		userChecker:                 userChecker,
		errorWrapper:                errors.NewServiceRecordNotFoundWrapper(),
		factoryUser2FAConfirmAction: factoryUser2FAConfirmAction,
		logOperation:                logOperation,
		realm2operation:             realm2operation,
	}
}

// Execute - проверяет логин пользователя в рамках realm, создаёт операцию создания
// сессии и в той же транзакции отправляет пользователю код её подтверждения.
func (co *CreateSession) Execute(
	ctx context.Context,
	actor dto.ActorMeta,
	realm, langCode, userLogin string,
) (secureoperation.SecureOperation, error) {
	if userLogin == "" {
		return secureoperation.SecureOperation{}, errors.ErrIncorrectInputData.New("userLogin is empty")
	}

	opCreator, ok := co.realm2operation[realm]
	if !ok {
		return secureoperation.SecureOperation{}, errors.ErrIncorrectInputData.New("realm is unknown")
	}

	parsedLogin, err := contactaddress.Parse(userLogin)
	if err != nil {
		return secureoperation.SecureOperation{}, errors.ErrIncorrectInputData.New(err)
	}

	err = co.userChecker.CheckAvailabilityRealm(ctx, realm, parsedLogin)
	if err == nil {
		// логина не существует: фиксируем в журнале попытку входа по несуществующему логину
		// (операция не создана, поэтому её имя берётся у фабрики, а метод подтверждения неизвестен)
		co.logOperation.Log(
			ctx,
			actor.NewOperationLog(
				opCreator.Name(), confirmmethod.Unspecified, logstatus.Blocked, logreason.LoginNotExists,
			),
		)

		return secureoperation.SecureOperation{}, mrauth.ErrLoginNotExists
	}

	if !errors.Is(err, mrauth.ErrEmailAlreadyExists) && !errors.Is(err, mrauth.ErrPhoneAlreadyExists) {
		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	user2FA, err := co.factoryUser2FAConfirmAction.CreateByUserLogin(ctx, parsedLogin)
	if err != nil {
		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	op, err := opCreator.Create(user2FA, realm, langCode, parsedLogin)
	if err != nil {
		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	// владельцем операции входа является существующий пользователь, поэтому Open фиксирует
	// в журнале его, а не анонимного посетителя (в actor приходит uuid.Nil)
	err = co.opener.Open(ctx, actor, op, "confirm.create.session.by.email", conv.Group{"lang": langCode})
	if err != nil {
		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	return op, nil
}
