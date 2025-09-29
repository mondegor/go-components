package auth

import (
	"context"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrargs"
	"github.com/mondegor/go-sysmess/mrerr/mr"

	core "github.com/mondegor/go-components/internal"
	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/contactaddress"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// CreateSession - компонент для извлечения настроек, которые хранятся в хранилище данных.
	CreateSession struct {
		txManager             mrstorage.DBTxManager
		userChecker           mrauth.CheckUserUseCase
		storageOperation      mrauth.SecureOperationStorage
		notifierAPI           mrnotifier.NoticeProducer
		loginParser           loginParser
		factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA
		realm2operation       map[string]createSessionOperation
		errorWrapper          core.UseCaseErrorWrapper
	}

	// CreateSessionRealm - сообщение для получателя.
	CreateSessionRealm struct {
		Name      string
		Operation createSessionOperation
	}

	loginParser interface {
		Parse(value string) (contactaddress.ContactAddress, error)
	}

	createSessionOperation interface {
		Create(user2FA dto.User2FA, realm, langCode string, address contactaddress.ContactAddress) (entity.SecureOperation, error)
	}
)

// NewCreateSession - создаёт объект UserProvider.
func NewCreateSession(
	txManager mrstorage.DBTxManager,
	userChecker mrauth.CheckUserUseCase,
	storageOperation mrauth.SecureOperationStorage,
	notifierAPI mrnotifier.NoticeProducer,
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
		realm2operation:       realm2operation,
		factoryUserConfirm2FA: factoryUserConfirm2FA,
		errorWrapper:          core.NewUseCaseErrorWrapper(entity.ModelNameRefreshToken), // ??????
	}
}

// Perform - возвращает строковое значение настройки с указанным идентификатором.
func (co *CreateSession) Perform(ctx context.Context, realm, langCode, userLogin string) (entity.SecureOperation, error) {
	if userLogin == "" {
		return entity.SecureOperation{}, mr.ErrUseCaseIncorrectInputData.New("userLogin is empty")
	}

	operationCreator, ok := co.realm2operation[realm]
	if !ok {
		return entity.SecureOperation{}, mr.ErrUseCaseIncorrectInputData.New("realm is unknown", "realm", realm)
	}

	parsedLogin, err := co.loginParser.Parse(userLogin)
	if err != nil {
		return entity.SecureOperation{}, mr.ErrUseCaseIncorrectInputData.New(err)
	}

	err = co.userChecker.CheckAvailability(ctx, realm, parsedLogin.Value)
	if err == nil {
		return entity.SecureOperation{}, mrauth.ErrLoginNotExists.New()
	}

	if !mrauth.ErrEmailAlreadyExists.Is(err) && !mrauth.ErrPhoneAlreadyExists.Is(err) {
		return entity.SecureOperation{}, co.errorWrapper.WrapErrorFailed(err)
	}

	user2FA, err := co.factoryUserConfirm2FA.CreateByUserLogin(ctx, parsedLogin)
	if err != nil {
		return entity.SecureOperation{}, co.errorWrapper.WrapErrorFailed(err)
	}

	op, err := operationCreator.Create(user2FA, realm, langCode, parsedLogin)
	if err != nil {
		return entity.SecureOperation{}, co.errorWrapper.WrapErrorFailed(err)
	}

	err = co.txManager.Do(ctx, func(ctx context.Context) error {
		if err = co.storageOperation.Insert(ctx, op); err != nil {
			return co.errorWrapper.WrapErrorFailed(err)
		}

		confirmingAction, err := op.NextNotConfirmedAction()
		if err != nil {
			return co.errorWrapper.WrapErrorFailed(err)
		}

		if confirmingAction.Method != enum.ConfirmMethodEmail {
			return mr.ErrInternal.New("reason", "confirm operation method is not email")
		}

		// TODO: Add Operation log:op!

		return co.notifierAPI.SendNotice(
			ctx,
			"confirm.create.session.by.email",
			mrargs.Group{
				"lang":        langCode,
				"to":          user2FA.Email,
				"confirmCode": confirmingAction.Secret,
			},
		)
	})
	if err != nil {
		return entity.SecureOperation{}, co.errorWrapper.WrapErrorFailed(err)
	}

	return op, nil
}
