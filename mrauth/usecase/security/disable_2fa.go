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
	// Disable2FA - comment struct.
	Disable2FA struct {
		txManager                  mrstorage.DBTxManager
		storageOperation           mrauth.SecureOperationStorage
		notifierAPI                mrnotifier.NoticeProducer
		factoryUserConfirm2FA      mrauth.FactoryUserConfirm2FA
		factoryOperationDisable2FA factoryOperation2FA
		errorWrapper               mrerr.UseCaseErrorWrapper
	}
)

// NewDisable2FA - создаёт объект Disable2FA.
func NewDisable2FA(
	txManager mrstorage.DBTxManager,
	storageOperation mrauth.SecureOperationStorage,
	notifierAPI mrnotifier.NoticeProducer,
	factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA,
	factoryOperationDisable2FA factoryOperation2FA,
	errorWrapper mrerr.UseCaseErrorWrapper,
) *Disable2FA {
	return &Disable2FA{
		txManager:                  txManager,
		storageOperation:           storageOperation,
		notifierAPI:                notifierAPI,
		factoryUserConfirm2FA:      factoryUserConfirm2FA,
		factoryOperationDisable2FA: factoryOperationDisable2FA,
		errorWrapper:               mrerr.NewUseCaseErrorWrapper(errorWrapper, "mrauth.Disable2FA"),
	}
}

// Execute - comments method.
func (uc *Disable2FA) Execute(ctx context.Context, userID uuid.UUID) (entity.SecureOperation, error) {
	if userID == uuid.Nil {
		return entity.SecureOperation{}, mr.ErrUseCaseAccessForbidden.New() // TODO 401!!!!
	}

	user2FA, err := uc.factoryUserConfirm2FA.CreateByUserID(ctx, userID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	// if user2FA.Action2FA.Method == 0 {
	// 	return entity.SecureOperation{}, errors.New("already disabled")
	// }

	op, err := uc.factoryOperationDisable2FA.Create(user2FA)
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
			return uc.notifierAPI.SendNotice(ctx, "confirm.disable.2fa", mrargs.Group{"to": confirmingAction.Address, "confirmCode": confirmingAction.Secret})
		}

		return nil
	})
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	return op, nil
}
