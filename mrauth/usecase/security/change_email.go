package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-sysmess/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ChangeEmailProperty - comment struct.
	ChangeEmailProperty struct {
		txManager             mrstorage.DBTxManager
		storageOperation      operationCreator
		emailChecker          userEmailChecker
		notifierAPI           mrnotifier.NoteProducer
		factoryUserConfirm2FA mrauth.FactoryUserConfirm2FA
		factoryOperationEmail factoryOperationValue2FA
		errorWrapper          errors.Wrapper
	}

	operationCreator interface {
		Insert(ctx context.Context, row secureoperation.SecureOperation) error
	}

	userEmailChecker interface {
		CheckAvailabilityEmail(ctx context.Context, userEmail contactaddress.ContactAddress) error
	}

	factoryOperationValue2FA interface {
		Create(user2FA dto.User2FA, fieldValue string) (secureoperation.SecureOperation, error)
	}
)

// NewChangeEmailProperty - создаёт объект ChangeEmailProperty.
func NewChangeEmailProperty(
	txManager mrstorage.DBTxManager,
	storageOperation operationCreator,
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
		errorWrapper:          errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - comments method.
func (uc *ChangeEmailProperty) Execute(ctx context.Context, userID uuid.UUID, newEmail string) (secureoperation.SecureOperation, error) {
	if userID == uuid.Nil {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	parsedEmail, err := contactaddress.ParseEmail(newEmail)
	if err != nil {
		return secureoperation.SecureOperation{}, errors.ErrIncorrectInputData.New(err)
	}

	if err := uc.emailChecker.CheckAvailabilityEmail(ctx, parsedEmail); err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	user2FA, err := uc.factoryUserConfirm2FA.CreateByUserID(ctx, userID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	op, err := uc.factoryOperationEmail.Create(user2FA, newEmail)
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
					"confirm.change.email",
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
