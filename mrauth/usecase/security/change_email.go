package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/util/conv"

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
		notifierAPI           mrnotifier.NoteProducer
		factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA
		factoryOperationEmail factoryOperationValue2FA
		errorWrapper          errors.Wrapper
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
	notifierAPI mrnotifier.NoteProducer,
	factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA,
	factoryOperationEmail factoryOperationValue2FA,
) *ChangeEmailProperty {
	return &ChangeEmailProperty{
		txManager:             txManager,
		storageOperation:      storageOperation,
		emailChecker:          emailChecker,
		notifierAPI:           notifierAPI,
		factoryUserConfirm2FA: factoryUserConfirm2FA,
		factoryOperationEmail: factoryOperationEmail,
		errorWrapper:          errors.NewUseCaseWrapper(),
	}
}

// Execute - comments method.
func (uc *ChangeEmailProperty) Execute(ctx context.Context, userID uuid.UUID, newEmail string) (entity.SecureOperation, error) {
	if userID == uuid.Nil {
		return entity.SecureOperation{}, errors.ErrUseCaseAccessForbidden // TODO 401!!!!
	}

	if err := uc.emailChecker.CheckAvailabilityEmail(ctx, newEmail); err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	user2FA, err := uc.factoryUserConfirm2FA.CreateByUserID(ctx, userID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	op, err := uc.factoryOperationEmail.Create(user2FA, newEmail)
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err = uc.storageOperation.Insert(ctx, op); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		confirmingAction, err := op.NextNotConfirmedAction()
		if err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		// TODO: Add Operation log:op!

		if confirmingAction.MaxResends > 0 {
			return uc.notifierAPI.Send(ctx, "confirm.change.email", conv.Group{"to": confirmingAction.Address, "confirmCode": confirmingAction.Secret})
		}

		return nil
	})
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	return op, nil
}
