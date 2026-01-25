package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ChangePasswordProperty - comment struct.
	ChangePasswordProperty struct {
		txManager                mrstorage.DBTxManager
		storageOperation         mrauth.SecureOperationStorage
		notifierAPI              mrnotifier.NoteProducer
		factoryUserConfirm2FA    mrauth.FactoryUserConfirm2FA
		factoryOperationPassword factoryOperationValue2FA
		errorWrapper             errors.Wrapper
	}
)

// NewChangePasswordProperty - создаёт объект ChangePasswordProperty.
func NewChangePasswordProperty(
	txManager mrstorage.DBTxManager,
	storageOperation mrauth.SecureOperationStorage,
	notifierAPI mrnotifier.NoteProducer,
	factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA,
	factoryOperationPassword factoryOperationValue2FA,
) *ChangePasswordProperty {
	return &ChangePasswordProperty{
		txManager:                txManager,
		storageOperation:         storageOperation,
		notifierAPI:              notifierAPI,
		factoryUserConfirm2FA:    factoryUserConfirm2FA,
		factoryOperationPassword: factoryOperationPassword,
		errorWrapper:             errors.NewUseCaseWrapper(),
	}
}

// Execute - comments method.
func (uc *ChangePasswordProperty) Execute(ctx context.Context, userID uuid.UUID, newPassword string) (entity.SecureOperation, error) {
	if userID == uuid.Nil {
		return entity.SecureOperation{}, errors.ErrUseCaseAccessForbidden // TODO 401!!!!
	}

	user2FA, err := uc.factoryUserConfirm2FA.CreateByUserID(ctx, userID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	op, err := uc.factoryOperationPassword.Create(user2FA, newPassword)
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
			return uc.notifierAPI.Send(
				ctx,
				"confirm.change.password",
				conv.Group{
					"to":          confirmingAction.Address,
					"confirmCode": confirmingAction.Secret,
				},
			)
		}

		return nil
	})
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	return op, nil
}
