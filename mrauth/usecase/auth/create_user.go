package auth

import (
	"context"
	"time"

	"github.com/mondegor/go-storage/mrlock"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrargs"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/contactaddress"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrnotifier"
)

const (
	createUserLockKeyPrefix = "auth.createUser:"
	createUserLockTimeout   = 10 * time.Minute
)

type (
	// CreateUser - компонент для извлечения настроек, которые хранятся в хранилище данных.
	CreateUser struct {
		txManager        mrstorage.DBTxManager
		userChecker      mrauth.CheckUserUseCase
		storageOperation mrauth.SecureOperationStorage
		notifierAPI      mrnotifier.NoticeProducer
		locker           mrlock.Locker
		loginParser      loginEmailParser
		errorWrapper     mrerr.UseCaseErrorWrapper
		realm2operation  map[string]createUserOperation
	}

	// CreateUserRealm - сообщение для получателя.
	CreateUserRealm struct {
		Name      string
		Operation createUserOperation
	}

	loginEmailParser interface {
		ParseEmail(value string) (contactaddress.ContactAddress, error)
	}

	createUserOperation interface {
		Create(langCode string, address contactaddress.ContactAddress) (entity.SecureOperation, error)
	}
)

// NewCreateUser - создаёт объект CreateUser.
func NewCreateUser(
	txManager mrstorage.DBTxManager,
	userChecker mrauth.CheckUserUseCase,
	storageOperation mrauth.SecureOperationStorage,
	notifierAPI mrnotifier.NoticeProducer,
	locker mrlock.Locker,
	loginParser loginEmailParser,
	errorWrapper mrerr.UseCaseErrorWrapper,
	allowedRealms []CreateUserRealm,
) *CreateUser {
	realm2operation := make(map[string]createUserOperation, len(allowedRealms))
	for _, realm := range allowedRealms {
		realm2operation[realm.Name] = realm.Operation
	}

	return &CreateUser{
		txManager:        txManager,
		userChecker:      userChecker,
		storageOperation: storageOperation,
		notifierAPI:      notifierAPI,
		locker:           locker,
		loginParser:      loginParser,
		errorWrapper:     mrerr.NewUseCaseErrorWrapper(errorWrapper, entity.ModelNameUser),
		realm2operation:  realm2operation,
	}
}

// Perform - возвращает строковое значение настройки с указанным идентификатором.
func (co *CreateUser) Perform(ctx context.Context, realm, langCode, userEmail string) (op entity.SecureOperation, err error) {
	operationCreator, ok := co.realm2operation[realm]
	if !ok {
		return entity.SecureOperation{}, mr.ErrUseCaseIncorrectInputData.New("realm is unknown", "realm", realm)
	}

	parsedLogin, err := co.loginParser.ParseEmail(userEmail)
	if err != nil {
		return entity.SecureOperation{}, mr.ErrUseCaseIncorrectInputData.New(err, "userEmail", userEmail)
	}

	unlockEmail, err := co.locker.LockWithExpiry(ctx, createUserLockKeyPrefix+realm+":"+parsedLogin.Value, createUserLockTimeout)
	if err != nil {
		if mrlock.ErrStorageLockKeyNotObtained.Is(err) {
			return entity.SecureOperation{}, mrauth.ErrEmailAlreadyExists.New()
		}

		return entity.SecureOperation{}, co.errorWrapper.WrapErrorFailed(err)
	}

	defer func() {
		// если в процессе выполнения метода возникла ошибка,
		// то емаил освобождается
		if err != nil {
			unlockEmail()
		}
	}()

	if err = co.userChecker.CheckAvailability(ctx, realm, parsedLogin.Value); err != nil {
		return entity.SecureOperation{}, co.errorWrapper.WrapErrorFailed(err)
	}

	op, err = operationCreator.Create(langCode, parsedLogin)
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

		// TODO: Add Operation log:op!

		return co.notifierAPI.SendNotice(
			ctx,
			"confirm.user.activation",
			mrargs.Group{
				"lang":        langCode,
				"to":          parsedLogin.Value,
				"confirmCode": confirmingAction.Secret,
			},
		)
	})
	if err != nil {
		return entity.SecureOperation{}, co.errorWrapper.WrapErrorFailed(err)
	}

	return op, nil
}
