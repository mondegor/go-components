package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ChangePhoneProperty - comment struct.
	ChangePhoneProperty struct {
		txManager             mrstorage.DBTxManager
		storageOperation      operationCreator
		phoneChecker          userPhoneChecker
		notifierAPI           mrnotifier.NoteProducer
		factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA
		factoryOperationPhone factoryOperationValue2FA
		errorWrapper          errors.Wrapper
	}

	userPhoneChecker interface {
		CheckAvailabilityPhone(ctx context.Context, userPhone contactaddress.ContactAddress) error
	}
)

// NewChangePhoneProperty - создаёт объект ChangePhoneProperty.
func NewChangePhoneProperty(
	txManager mrstorage.DBTxManager,
	storageOperation operationCreator,
	phoneChecker userPhoneChecker,
	notifierAPI mrnotifier.NoteProducer,
	factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA,
	factoryOperationPhone factoryOperationValue2FA,
) *ChangePhoneProperty {
	return &ChangePhoneProperty{
		txManager:             txManager,
		storageOperation:      storageOperation,
		phoneChecker:          phoneChecker,
		notifierAPI:           notifierAPI,
		factoryUserConfirm2FA: factoryUserConfirm2FA,
		factoryOperationPhone: factoryOperationPhone,
		errorWrapper:          errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - comments method.
func (uc *ChangePhoneProperty) Execute(ctx context.Context, userID uuid.UUID, newPhone string) (secureoperation.SecureOperation, error) {
	if userID == uuid.Nil {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	parsedPhone, err := contactaddress.ParsePhone(newPhone)
	if err != nil {
		return secureoperation.SecureOperation{}, errors.ErrIncorrectInputData.New(err)
	}

	if err := uc.phoneChecker.CheckAvailabilityPhone(ctx, parsedPhone); err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	user2FA, err := uc.factoryUserConfirm2FA.CreateByUserID(ctx, userID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	op, err := uc.factoryOperationPhone.Create(user2FA, newPhone)
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
					"confirm.change.phone",
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
