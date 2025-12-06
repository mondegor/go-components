package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrargs"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ChangeEmailProperty - comment struct.
	ChangeEmailProperty struct {
		txManager             mrstorage.DBTxManager
		storageOperation      mrauth.SecureOperationStorage
		emailChecker          userEmailChecker
		notifierAPI           mrnotifier.NoticeProducer
		factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA
		factoryOperationEmail factoryOperationValue2FA
		errorWrapper          mrerr.UseCaseErrorWrapper
	}

	userEmailChecker interface {
		CheckAvailabilityEmail(ctx context.Context, userEmail string) error
	}

	factoryOperationValue2FA interface {
		Create(user2FA dto.User2FA, fieldValue string) (entity.SecureOperation, error)
	}
)

// NewChangeEmailProperty - создаёт объект ChangeEmailProperty.
func NewChangeEmailProperty(
	txManager mrstorage.DBTxManager,
	storageOperation mrauth.SecureOperationStorage,
	emailChecker userEmailChecker,
	notifierAPI mrnotifier.NoticeProducer,
	factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA,
	factoryOperationEmail factoryOperationValue2FA,
	errorWrapper mrerr.UseCaseErrorWrapper,
) *ChangeEmailProperty {
	return &ChangeEmailProperty{
		txManager:             txManager,
		storageOperation:      storageOperation,
		emailChecker:          emailChecker,
		notifierAPI:           notifierAPI,
		factoryUserConfirm2FA: factoryUserConfirm2FA,
		factoryOperationEmail: factoryOperationEmail,
		errorWrapper:          mrerr.NewUseCaseErrorWrapper(errorWrapper, "mrauth.ChangeEmailProperty"),
	}
}

// Execute - comments method.
func (uc *ChangeEmailProperty) Execute(ctx context.Context, userID uuid.UUID, newEmail string) (entity.SecureOperation, error) {
	if userID == uuid.Nil {
		return entity.SecureOperation{}, mr.ErrUseCaseAccessForbidden.New() // TODO 401!!!!
	}

	if err := uc.emailChecker.CheckAvailabilityEmail(ctx, newEmail); err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	user2FA, err := uc.factoryUserConfirm2FA.CreateByUserID(ctx, userID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	op, err := uc.factoryOperationEmail.Create(user2FA, newEmail)
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
			return uc.notifierAPI.SendNotice(ctx, "confirm.change.email", mrargs.Group{"to": confirmingAction.Address, "confirmCode": confirmingAction.Secret})
		}

		return nil
	})
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	return op, nil
}
