package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ChangeTOTPGeneratorProperty - comment struct.
	ChangeTOTPGeneratorProperty struct {
		txManager             mrstorage.DBTxManager
		storageOperation      operationCreator
		notifierAPI           mrnotifier.NoteProducer
		factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA
		factoryOperationTOTP  factoryOperation2FA
		errorWrapper          errors.Wrapper
	}

	factoryOperation2FA interface {
		Create(user2FA dto.User2FA) (secureoperation.SecureOperation, error)
	}
)

// NewChangeTOTPGeneratorProperty - создаёт объект ChangeTOTPGeneratorProperty.
func NewChangeTOTPGeneratorProperty(
	txManager mrstorage.DBTxManager,
	storageOperation operationCreator,
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
		errorWrapper:          errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - comments method.
func (uc *ChangeTOTPGeneratorProperty) Execute(ctx context.Context, userID uuid.UUID) (secureoperation.SecureOperation, error) {
	if userID == uuid.Nil {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	user2FA, err := uc.factoryUserConfirm2FA.CreateByUserID(ctx, userID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	op, err := uc.factoryOperationTOTP.Create(user2FA)
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err = uc.storageOperation.Insert(ctx, op); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		// TODO: Add Operation log:op!

		return op.Notify(
			func(method confirmmethod.Enum, address, confirmCode string) error {
				if method != confirmmethod.Email {
					return errors.NewInternalError("ConfirmMethod is not yet supported", "method", method)
				}

				return uc.notifierAPI.Send(
					ctx,
					"confirm.change.totp",
					conv.Group{
						"to":          address,
						"confirmCode": confirmCode,
					},
				)
			},
		)
	})
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	return op, nil
}
