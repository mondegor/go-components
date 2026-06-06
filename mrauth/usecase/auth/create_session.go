package auth

import (
	"context"

	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-sysmess/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// CreateSession - компонент для извлечения настроек, которые хранятся в хранилище данных.
	CreateSession struct {
		txManager             mrstorage.DBTxManager
		userChecker           userLoginChecker
		storageOperation      operationCreator
		notifierAPI           mrnotifier.NoteProducer
		factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA
		errorWrapper          errors.Wrapper
		realm2operation       map[string]createSessionOperation
	}

	// CreateSessionRealm - сообщение для получателя.
	CreateSessionRealm struct {
		Name      string
		Operation createSessionOperation
	}

	operationCreator interface {
		Insert(ctx context.Context, row secureoperation.SecureOperation) error
	}

	userLoginChecker interface {
		CheckAvailabilityRealm(ctx context.Context, realm string, userLogin contactaddress.ContactAddress) error
	}

	createSessionOperation interface {
		Create(user2FA dto.User2FA, realm, langCode string, address contactaddress.ContactAddress) (secureoperation.SecureOperation, error)
	}
)

// NewCreateSession - создаёт объект UserProvider.
func NewCreateSession(
	txManager mrstorage.DBTxManager,
	userChecker userLoginChecker,
	storageOperation operationCreator,
	notifierAPI mrnotifier.NoteProducer,
	factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA,
	allowedRealms []CreateSessionRealm,
) *CreateSession {
	realm2operation := make(map[string]createSessionOperation, len(allowedRealms))
	for _, realm := range allowedRealms {
		realm2operation[realm.Name] = realm.Operation
	}

	return &CreateSession{
		txManager:             txManager,
		userChecker:           userChecker,
		storageOperation:      storageOperation,
		notifierAPI:           notifierAPI,
		errorWrapper:          errors.NewServiceRecordNotFoundWrapper(),
		factoryUserConfirm2FA: factoryUserConfirm2FA,
		realm2operation:       realm2operation,
	}
}

// Execute - возвращает строковое значение настройки с указанным идентификатором.
func (co *CreateSession) Execute(ctx context.Context, realm, langCode, userLogin string) (secureoperation.SecureOperation, error) {
	if userLogin == "" {
		return secureoperation.SecureOperation{}, errors.ErrIncorrectInputData.New("userLogin is empty")
	}

	operationCreator, ok := co.realm2operation[realm]
	if !ok {
		return secureoperation.SecureOperation{}, errors.ErrIncorrectInputData.New("realm is unknown")
	}

	parsedLogin, err := contactaddress.Parse(userLogin)
	if err != nil {
		return secureoperation.SecureOperation{}, errors.ErrIncorrectInputData.New(err)
	}

	err = co.userChecker.CheckAvailabilityRealm(ctx, realm, parsedLogin)
	if err == nil {
		return secureoperation.SecureOperation{}, mrauth.ErrLoginNotExists
	}

	if !errors.Is(err, mrauth.ErrEmailAlreadyExists) && !errors.Is(err, mrauth.ErrPhoneAlreadyExists) {
		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	user2FA, err := co.factoryUserConfirm2FA.CreateByUserLogin(ctx, parsedLogin)
	if err != nil {
		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	op, err := operationCreator.Create(user2FA, realm, langCode, parsedLogin)
	if err != nil {
		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	err = co.txManager.Do(ctx, func(ctx context.Context) error {
		if err = co.storageOperation.Insert(ctx, op); err != nil {
			return co.errorWrapper.Wrap(err)
		}

		// TODO: Add Operation log:op!

		return op.Notify(
			func(method confirmmethod.Enum, address, confirmCode string) error {
				if method != confirmmethod.Email {
					return errors.NewInternalError("ConfirmMethod is not yet supported", "method", method)
				}

				return co.notifierAPI.Send(
					ctx,
					"confirm.create.session.by.email",
					conv.Group{
						"lang":        langCode,
						"to":          address, // user2FA.Email
						"confirmCode": confirmCode,
					},
				)
			},
		)
	})
	if err != nil {
		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	return op, nil
}
