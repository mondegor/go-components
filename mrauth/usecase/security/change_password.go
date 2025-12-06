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
	// ChangePasswordProperty - comment struct.
	ChangePasswordProperty struct {
		txManager                mrstorage.DBTxManager
		storageOperation         mrauth.SecureOperationStorage
		notifierAPI              mrnotifier.NoticeProducer
		factoryUserConfirm2FA    mrauth.FactoryUserConfirm2FA
		factoryOperationPassword factoryOperationValue2FA
		errorWrapper             mrerr.UseCaseErrorWrapper
	}
)

// NewChangePasswordProperty - создаёт объект ChangePasswordProperty.
func NewChangePasswordProperty(
	txManager mrstorage.DBTxManager,
	storageOperation mrauth.SecureOperationStorage,
	notifierAPI mrnotifier.NoticeProducer,
	factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA,
	factoryOperationPassword factoryOperationValue2FA,
	errorWrapper mrerr.UseCaseErrorWrapper,
) *ChangePasswordProperty {
	return &ChangePasswordProperty{
		txManager:                txManager,
		storageOperation:         storageOperation,
		notifierAPI:              notifierAPI,
		factoryUserConfirm2FA:    factoryUserConfirm2FA,
		factoryOperationPassword: factoryOperationPassword,
		errorWrapper:             mrerr.NewUseCaseErrorWrapper(errorWrapper, "mrauth.ChangePasswordProperty"),
	}
}

// Execute - comments method.
func (uc *ChangePasswordProperty) Execute(ctx context.Context, userID uuid.UUID, newPassword string) (entity.SecureOperation, error) {
	if userID == uuid.Nil {
		return entity.SecureOperation{}, mr.ErrUseCaseAccessForbidden.New() // TODO 401!!!!
	}

	user2FA, err := uc.factoryUserConfirm2FA.CreateByUserID(ctx, userID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	op, err := uc.factoryOperationPassword.Create(user2FA, newPassword)
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
			return uc.notifierAPI.SendNotice(
				ctx,
				"confirm.change.password",
				mrargs.Group{
					"to":          confirmingAction.Address,
					"confirmCode": confirmingAction.Secret,
				},
			)
		}

		return nil
	})
	if err != nil {
		return entity.SecureOperation{}, uc.errorWrapper.WrapErrorFailed(err)
	}

	return op, nil
}
