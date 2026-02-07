package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/util/operation"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// Disable2FA - comment struct.
	Disable2FA struct {
		txManager                  mrstorage.DBTxManager
		storageOperation           operationCreator
		notifierAPI                mrnotifier.NoteProducer
		factoryUserConfirm2FA      mrauth.FactoryUserConfirm2FA
		factoryOperationDisable2FA factoryOperation2FA
		errorWrapper               errors.Wrapper
	}
)

// NewDisable2FA - создаёт объект Disable2FA.
func NewDisable2FA(
	txManager mrstorage.DBTxManager,
	storageOperation operationCreator,
	notifierAPI mrnotifier.NoteProducer,
	factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA,
	factoryOperationDisable2FA factoryOperation2FA,
) *Disable2FA {
	return &Disable2FA{
		txManager:                  txManager,
		storageOperation:           storageOperation,
		notifierAPI:                notifierAPI,
		factoryUserConfirm2FA:      factoryUserConfirm2FA,
		factoryOperationDisable2FA: factoryOperationDisable2FA,
		errorWrapper:               errors.NewUseCaseWrapper(),
	}
}

// Execute - comments method.
func (uc *Disable2FA) Execute(ctx context.Context, userID uuid.UUID) (secureoperation.SecureOperation, error) {
	if userID == uuid.Nil {
		return secureoperation.SecureOperation{}, errors.ErrUseCaseAccessForbidden // TODO 401!!!!
	}

	user2FA, err := uc.factoryUserConfirm2FA.CreateByUserID(ctx, userID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	// if user2FA.Action2FA.Method == 0 {
	// 	return entity.SecureOperation{}, errors.New("already disabled")
	// }

	op, err := uc.factoryOperationDisable2FA.Create(user2FA)
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err = uc.storageOperation.Insert(ctx, op); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		confirmingAction, err := operation.NextConfirmingAction(&op)
		if err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		// TODO: Add Operation log:op!

		if confirmingAction.MaxResends > 0 {
			return uc.notifierAPI.Send(
				ctx,
				"confirm.disable.2fa",
				conv.Group{
					"to":          confirmingAction.Address,
					"confirmCode": confirmingAction.Secret,
				},
			)
		}

		return nil
	})
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	return op, nil
}
