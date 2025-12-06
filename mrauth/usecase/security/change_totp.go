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
	// ChangeTOTPGeneratorProperty - comment struct.
	ChangeTOTPGeneratorProperty struct {
		txManager             mrstorage.DBTxManager
		storageOperation      mrauth.SecureOperationStorage
		notifierAPI           mrnotifier.NoticeProducer
		factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA
		factoryOperationTOTP  factoryOperation2FA
		errorWrapper          mrerr.UseCaseErrorWrapper
	}

	factoryOperation2FA interface {
		Create(user2FA dto.User2FA) (entity.SecureOperation, error)
	}
)

// NewChangeTOTPGeneratorProperty - создаёт объект ChangeTOTPGeneratorProperty.
func NewChangeTOTPGeneratorProperty(
	txManager mrstorage.DBTxManager,
	storageOperation mrauth.SecureOperationStorage,
	notifierAPI mrnotifier.NoticeProducer,
	factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA,
	factoryOperationTOTP factoryOperation2FA,
	errorWrapper mrerr.UseCaseErrorWrapper,
) *ChangeTOTPGeneratorProperty {
	return &ChangeTOTPGeneratorProperty{
		txManager:             txManager,
		storageOperation:      storageOperation,
		notifierAPI:           notifierAPI,
		factoryUserConfirm2FA: factoryUserConfirm2FA,
		factoryOperationTOTP:  factoryOperationTOTP,
		errorWrapper:          mrerr.NewUseCaseErrorWrapper(errorWrapper, "mrauth.ChangeTOTPGeneratorProperty"),
	}
}

// Execute - comments method.
func (uc *ChangeTOTPGeneratorProperty) Execute(ctx context.Context, userID uuid.UUID) (entity.SecureOperation, error) {
	if userID == uuid.Nil {
		return entity.SecureOperation{}, mr.ErrUseCaseAccessForbidden.New() // TODO 401!!!!
	}

	user2FA, err := uc.factoryUserConfirm2FA.CreateByUserID(ctx, userID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	op, err := uc.factoryOperationTOTP.Create(user2FA)
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
			return uc.notifierAPI.SendNotice(ctx, "confirm.change.totp", mrargs.Group{"to": confirmingAction.Address, "confirmCode": confirmingAction.Secret})
		}

		return nil
	})
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	return op, nil
}
