package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ChangePasswordProperty - создаёт операцию смены пароля пользователя и отправляет
	// код её подтверждения.
	ChangePasswordProperty struct {
		txManager                   mrstorage.DBTxManager
		storageOperation            operationCreator
		notifierAPI                 mrnotifier.NoteProducer
		factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator
		factoryOperationPassword    factoryOperationValue2FA
		errorWrapper                errors.Wrapper
	}
)

// NewChangePasswordProperty - создаёт объект ChangePasswordProperty.
func NewChangePasswordProperty(
	txManager mrstorage.DBTxManager,
	storageOperation operationCreator,
	notifierAPI mrnotifier.NoteProducer,
	factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator,
	factoryOperationPassword factoryOperationValue2FA,
) *ChangePasswordProperty {
	return &ChangePasswordProperty{
		txManager:                   txManager,
		storageOperation:            storageOperation,
		notifierAPI:                 notifierAPI,
		factoryUser2FAConfirmAction: factoryUser2FAConfirmAction,
		factoryOperationPassword:    factoryOperationPassword,
		errorWrapper:                errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - создаёт операцию смены пароля и в той же транзакции отправляет
// пользователю код её подтверждения.
func (uc *ChangePasswordProperty) Execute(ctx context.Context, userID uuid.UUID, newPassword string) (secureoperation.SecureOperation, error) {
	if userID == uuid.Nil {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	user2FA, err := uc.factoryUser2FAConfirmAction.CreateByUserID(ctx, userID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	// активный 2FA нельзя менять на месте: сначала нужно отключить текущий (disable 2FA)
	if user2FA.Action2FA.Method > 0 {
		return secureoperation.SecureOperation{}, mrauth.Err2FAMustBeDisabledFirst
	}

	op, err := uc.factoryOperationPassword.Create(user2FA, newPassword)
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err = uc.storageOperation.Insert(ctx, op); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		return op.NotifyByEmail(
			func(address, confirmCode string) error {
				return uc.notifierAPI.Send(
					ctx,
					"confirm.change.password",
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
