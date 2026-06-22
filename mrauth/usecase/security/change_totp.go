package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-sysmess/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ChangeTOTPGeneratorProperty - создаёт операцию смены TOTP-генератора пользователя
	// и отправляет код её подтверждения.
	ChangeTOTPGeneratorProperty struct {
		txManager                   mrstorage.DBTxManager
		storageOperation            operationCreator
		notifierAPI                 mrnotifier.NoteProducer
		factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator
		factoryOperationTOTP        user2faOperationCreator
		errorWrapper                errors.Wrapper
	}

	user2faOperationCreator interface {
		Create(user2FA dto.User2FA) (secureoperation.SecureOperation, error)
	}
)

// NewChangeTOTPGeneratorProperty - создаёт объект ChangeTOTPGeneratorProperty.
func NewChangeTOTPGeneratorProperty(
	txManager mrstorage.DBTxManager,
	storageOperation operationCreator,
	notifierAPI mrnotifier.NoteProducer,
	factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator,
	factoryOperationTOTP user2faOperationCreator,
) *ChangeTOTPGeneratorProperty {
	return &ChangeTOTPGeneratorProperty{
		txManager:                   txManager,
		storageOperation:            storageOperation,
		notifierAPI:                 notifierAPI,
		factoryUser2FAConfirmAction: factoryUser2FAConfirmAction,
		factoryOperationTOTP:        factoryOperationTOTP,
		errorWrapper:                errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - создаёт операцию смены TOTP-генератора и в той же транзакции отправляет
// пользователю код её подтверждения.
func (uc *ChangeTOTPGeneratorProperty) Execute(ctx context.Context, userID uuid.UUID) (secureoperation.SecureOperation, error) {
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

	op, err := uc.factoryOperationTOTP.Create(user2FA)
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err = uc.storageOperation.Insert(ctx, op); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		// TODO: записать операцию в журнал

		return op.NotifyByEmail(
			func(address, confirmCode string) error {
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
