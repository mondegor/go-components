package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrargs"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ChangePhoneProperty - comment struct.
	ChangePhoneProperty struct {
		txManager             mrstorage.DBTxManager
		storageOperation      mrauth.SecureOperationStorage
		phoneChecker          userPhoneChecker
		notifierAPI           mrnotifier.NoticeProducer
		factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA
		factoryOperationPhone factoryOperationValue2FA
		errorWrapper          mrerr.UseCaseErrorWrapper
	}

	userPhoneChecker interface {
		CheckAvailabilityPhone(ctx context.Context, userPhone string) error
	}
)

// NewChangePhoneProperty - создаёт объект ChangePhoneProperty.
func NewChangePhoneProperty(
	txManager mrstorage.DBTxManager,
	storageOperation mrauth.SecureOperationStorage,
	phoneChecker userPhoneChecker,
	notifierAPI mrnotifier.NoticeProducer,
	factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA,
	factoryOperationPhone factoryOperationValue2FA,
	errorWrapper mrerr.UseCaseErrorWrapper,
) *ChangePhoneProperty {
	return &ChangePhoneProperty{
		txManager:             txManager,
		storageOperation:      storageOperation,
		phoneChecker:          phoneChecker,
		notifierAPI:           notifierAPI,
		factoryUserConfirm2FA: factoryUserConfirm2FA,
		factoryOperationPhone: factoryOperationPhone,
		errorWrapper:          mrerr.NewUseCaseErrorWrapper(errorWrapper, "mrauth.ChangePhoneProperty"),
	}
}

// Execute - comments method.
func (uc *ChangePhoneProperty) Execute(ctx context.Context, userID uuid.UUID, newPhone string) (entity.SecureOperation, error) {
	if userID == uuid.Nil {
		return entity.SecureOperation{}, mr.ErrUseCaseAccessForbidden.New() // TODO 401!!!!
	}

	// TODO: проверить валидный ли телефон

	if err := uc.phoneChecker.CheckAvailabilityPhone(ctx, newPhone); err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	user2FA, err := uc.factoryUserConfirm2FA.CreateByUserID(ctx, userID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	op, err := uc.factoryOperationPhone.Create(user2FA, newPhone)
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err = uc.storageOperation.Insert(ctx, op); err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		confirmingAction, err := op.NextNotConfirmedAction()
		if err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		// TODO: Add Operation log:op!

		if confirmingAction.MaxResends > 0 {
			return uc.notifierAPI.SendNotice(ctx, "confirm.change.phone", mrargs.Group{"to": confirmingAction.Address, "confirmCode": confirmingAction.Secret})
		}

		return nil
	})
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	return op, nil
}
