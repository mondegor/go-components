package auth

import (
	"context"
	"time"

	"github.com/mondegor/go-storage/mrlock"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/util/conv"

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
	// CreateUser - comment struct.
	CreateUser struct {
		txManager        mrstorage.DBTxManager
		userChecker      userLoginChecker
		storageOperation mrauth.SecureOperationStorage
		notifierAPI      mrnotifier.NoteProducer
		locker           mrlock.Locker
		loginParser      loginEmailParser
		errorWrapper     errors.Wrapper
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
	userChecker userLoginChecker,
	storageOperation mrauth.SecureOperationStorage,
	notifierAPI mrnotifier.NoteProducer,
	locker mrlock.Locker,
	loginParser loginEmailParser,
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
		errorWrapper:     errors.NewUseCaseWrapper(),
		realm2operation:  realm2operation,
	}
}

// Execute - возвращает строковое значение настройки с указанным идентификатором.
func (co *CreateUser) Execute(ctx context.Context, realm, langCode, userEmail string) (op entity.SecureOperation, err error) {
	operationCreator, ok := co.realm2operation[realm]
	if !ok {
		return entity.SecureOperation{}, errors.ErrUseCaseIncorrectInputData.New("realm is unknown")
	}

	parsedLogin, err := co.loginParser.ParseEmail(userEmail)
	if err != nil {
		return entity.SecureOperation{}, errors.ErrUseCaseIncorrectInputData.New(err)
	}

	unlockEmail, err := co.locker.LockWithExpiry(ctx, createUserLockKeyPrefix+realm+":"+parsedLogin.Value, createUserLockTimeout)
	if err != nil {
		if errors.Is(err, mrlock.ErrSystemStorageLockKeyNotObtained) {
			return entity.SecureOperation{}, mrauth.ErrEmailAlreadyExists
		}

		return entity.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	defer func() {
		// если в процессе выполнения метода возникла ошибка,
		// то емаил освобождается
		if err != nil {
			unlockEmail()
		}
	}()

	if err = co.userChecker.CheckAvailabilityRealm(ctx, realm, parsedLogin); err != nil {
		return entity.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	op, err = operationCreator.Create(langCode, parsedLogin)
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

		// TODO: Add Operation log:op!

		return co.notifierAPI.Send(
			ctx,
			"confirm.user.activation",
			conv.Group{
				"lang":        langCode,
				"to":          parsedLogin.Value,
				"confirmCode": confirmingAction.Secret,
			},
		)
	})
	if err != nil {
		return entity.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	return op, nil
}
