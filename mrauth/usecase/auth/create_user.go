package auth

import (
	"context"
	"time"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrlock"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/mrtype"
	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrnotifier"
)

const (
	createUserLockKeyPrefix = "auth.create-user:"
	createUserLockTimeout   = 10 * time.Minute
)

type (
	// CreateUser - usecase создания пользователя с подтверждением через защищённую операцию.
	CreateUser struct {
		txManager        mrstorage.DBTxManager
		userChecker      userLoginChecker
		storageOperation operationCreator
		notifierAPI      mrnotifier.NoteProducer
		factory2FA       user2faActionCreator
		locker           mrlock.Locker
		errorWrapper     errors.Wrapper
		realm2operation  map[string]createUserOperation
	}

	// CreateUserRealm - сообщение для получателя.
	CreateUserRealm struct {
		Name      string
		Operation createUserOperation
	}

	createUserOperation interface {
		Create(user2FA dto.User2FA, langCode string, address contactaddress.ContactAddress, registeredIP string) (secureoperation.SecureOperation, error)
	}

	user2faActionCreator interface {
		CreateByUserLogin(ctx context.Context, userLogin contactaddress.ContactAddress) (dto.User2FA, error)
	}
)

// NewCreateUser - создаёт объект CreateUser.
func NewCreateUser(
	txManager mrstorage.DBTxManager,
	userChecker userLoginChecker,
	storageOperation operationCreator,
	notifierAPI mrnotifier.NoteProducer,
	factory2FA user2faActionCreator,
	locker mrlock.Locker,
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
		factory2FA:       factory2FA,
		locker:           locker,
		errorWrapper:     errors.NewServiceRecordNotFoundWrapper(),
		realm2operation:  realm2operation,
	}
}

// Execute - инициирует создание пользователя: открывает защищённую операцию подтверждения по коду
// и отправляет код на email. registeredIP фиксируется в payload операции как IP регистрации.
func (co *CreateUser) Execute(
	ctx context.Context,
	realm, langCode, userEmail string,
	registeredIP mrtype.DetailedIP,
) (op secureoperation.SecureOperation, err error) {
	opCreator, ok := co.realm2operation[realm]
	if !ok {
		return secureoperation.SecureOperation{}, errors.ErrIncorrectInputData.New("realm is unknown")
	}

	parsedLogin, err := contactaddress.ParseEmail(userEmail)
	if err != nil {
		return secureoperation.SecureOperation{}, errors.ErrIncorrectInputData.New(err)
	}

	// лок держится до createUserLockTimeout и НЕ освобождается при успехе - это намеренный
	// анти-спам троттл повторной отправки кода подтверждения на тот же email
	unlockEmail, err := co.locker.LockWithExpiry(ctx, createUserLockKeyPrefix+realm+":"+parsedLogin.Value(), createUserLockTimeout)
	if err != nil {
		if errors.Is(err, mrlock.ErrLockKeyNotObtained) {
			return secureoperation.SecureOperation{}, mrauth.ErrSignupAlreadyInProgressTryLater
		}

		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	defer func() {
		// если в процессе выполнения метода возникла ошибка,
		// то емаил освобождается
		if err != nil {
			unlockEmail()
		}
	}()

	if err = co.userChecker.CheckAvailabilityRealm(ctx, realm, parsedLogin); err != nil {
		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	// если email уже принадлежит существующему пользователю, его 2FA будет добавлен вторым шагом
	// операции (для нового email пользователь не найден и используется пустой User2FA)
	user2FA, err := co.factory2FA.CreateByUserLogin(ctx, parsedLogin)
	if err != nil {
		if !errors.Is(err, errors.ErrEventStorageNoRecordFound) {
			return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
		}
	}

	op, err = opCreator.Create(user2FA, langCode, parsedLogin, registeredIP.String())
	if err != nil {
		return secureoperation.SecureOperation{}, co.errorWrapper.Wrap(err)
	}

	err = co.txManager.Do(ctx, func(ctx context.Context) error {
		if err = co.storageOperation.Insert(ctx, op); err != nil {
			return co.errorWrapper.Wrap(err)
		}

		// TODO: записать операцию в журнал

		return op.NotifyByEmail(
			func(address, confirmCode string) error {
				return co.notifierAPI.Send(
					ctx,
					"confirm.user.activation",
					conv.Group{
						"lang":        langCode,
						"to":          address, // parsedLogin.Value()
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
