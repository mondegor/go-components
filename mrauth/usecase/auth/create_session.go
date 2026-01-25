package auth

import (
	"context"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/contactaddress"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// CreateSession - компонент для извлечения настроек, которые хранятся в хранилище данных.
	CreateSession struct {
		txManager             mrstorage.DBTxManager
		userChecker           userLoginChecker
		storageOperation      mrauth.SecureOperationStorage
		notifierAPI           mrnotifier.NoteProducer
		loginParser           loginParser
		factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA
		errorWrapper          errors.Wrapper
		realm2operation       map[string]createSessionOperation
	}

	// CreateSessionRealm - сообщение для получателя.
	CreateSessionRealm struct {
		Name      string
		Operation createSessionOperation
	}

	loginParser interface {
		Parse(value string) (contactaddress.ContactAddress, error)
	}

	userLoginChecker interface {
		CheckAvailabilityRealm(ctx context.Context, realm string, userLogin contactaddress.ContactAddress) error
	}

	createSessionOperation interface {
		Create(user2FA dto.User2FA, realm, langCode string, address contactaddress.ContactAddress) (entity.SecureOperation, error)
	}
)

// NewCreateSession - создаёт объект UserProvider.
func NewCreateSession(
	txManager mrstorage.DBTxManager,
	userChecker userLoginChecker,
	storageOperation mrauth.SecureOperationStorage,
	notifierAPI mrnotifier.NoteProducer,
	loginParser loginParser,
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
		loginParser:           loginParser,
		errorWrapper:          errors.NewUseCaseWrapper(),
		factoryUserConfirm2FA: factoryUserConfirm2FA,
		realm2operation:       realm2operation,
	}
}

// Execute - возвращает строковое значение настройки с указанным идентификатором.
func (co *CreateSession) Execute(ctx context.Context, realm, langCode, userLogin string) (entity.SecureOperation, error) {
	if userLogin == "" {
		return entity.SecureOperation{}, errors.ErrUseCaseIncorrectInputData.New("userLogin is empty")
	}

	operationCreator, ok := co.realm2operation[realm]
	if !ok {
		return entity.SecureOperation{}, errors.ErrUseCaseIncorrectInputData.New("realm is unknown")
	}

	parsedLogin, err := co.loginParser.Parse(userLogin)
	if err != nil {
		return entity.SecureOperation{}, errors.ErrUseCaseIncorrectInputData.New(err)
	}

	err = co.userChecker.CheckAvailabilityRealm(ctx, realm, parsedLogin)
	if err == nil {
		return entity.SecureOperation{}, mrauth.ErrLoginNotExists
	}

	if !errors.Is(err, mrauth.ErrEmailAlreadyExists) && !errors.Is(err, mrauth.ErrPhoneAlreadyExists) {
		return entity.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	user2FA, err := co.factoryUserConfirm2FA.CreateByUserLogin(ctx, parsedLogin)
	if err != nil {
		return entity.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	op, err := operationCreator.Create(user2FA, realm, langCode, parsedLogin)
	if err != nil {
		return entity.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	err = co.txManager.Do(ctx, func(ctx context.Context) error {
		if err = co.storageOperation.Insert(ctx, op); err != nil {
			return co.errorWrapper.Wrap(err)
		}

		confirmingAction, err := op.NextNotConfirmedAction()
		if err != nil {
			return co.errorWrapper.Wrap(err)
		}

		if confirmingAction.Method != confirmmethod.Email {
			return errors.NewInternalError("confirm operation method is not email")
		}

		// TODO: Add Operation log:op!

		return co.notifierAPI.Send(
			ctx,
			"confirm.create.session.by.email",
			conv.Group{
				"lang":        langCode,
				"to":          user2FA.Email,
				"confirmCode": confirmingAction.Secret,
			},
		)
	})
	if err != nil {
		return entity.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	return op, nil
}
