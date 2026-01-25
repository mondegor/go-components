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
	// ChangeTOTPGeneratorProperty - comment struct.
	ChangeTOTPGeneratorProperty struct {
		txManager             mrstorage.DBTxManager
		storageOperation      mrauth.SecureOperationStorage
		notifierAPI           mrnotifier.NoteProducer
		factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA
		factoryOperationTOTP  factoryOperation2FA
		errorWrapper          errors.Wrapper
	}

	factoryOperation2FA interface {
		Create(user2FA dto.User2FA) (entity.SecureOperation, error)
	}
)

// NewChangeTOTPGeneratorProperty - создаёт объект ChangeTOTPGeneratorProperty.
func NewChangeTOTPGeneratorProperty(
	txManager mrstorage.DBTxManager,
	storageOperation mrauth.SecureOperationStorage,
	notifierAPI mrnotifier.NoteProducer,
	factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA,
	factoryOperationTOTP factoryOperation2FA,
) *ChangeTOTPGeneratorProperty {
	return &ChangeTOTPGeneratorProperty{
		txManager:             txManager,
		storageOperation:      storageOperation,
		notifierAPI:           notifierAPI,
		factoryUserConfirm2FA: factoryUserConfirm2FA,
		factoryOperationTOTP:  factoryOperationTOTP,
		errorWrapper:          errors.NewUseCaseWrapper(),
	}
}

// Execute - comments method.
func (uc *ChangeTOTPGeneratorProperty) Execute(ctx context.Context, userID uuid.UUID) (entity.SecureOperation, error) {
	if userID == uuid.Nil {
		return entity.SecureOperation{}, errors.ErrUseCaseAccessForbidden // TODO 401!!!!
	}

	user2FA, err := uc.factoryUserConfirm2FA.CreateByUserID(ctx, userID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	op, err := uc.factoryOperationTOTP.Create(user2FA)
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
			return uc.notifierAPI.Send(ctx, "confirm.change.totp", conv.Group{"to": confirmingAction.Address, "confirmCode": confirmingAction.Secret})
		}

		return nil
	})
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	return op, nil
}
