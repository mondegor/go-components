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
	// Disable2FA - создаёт операцию отключения 2FA пользователя и отправляет код
	// её подтверждения.
	Disable2FA struct {
		txManager                   mrstorage.DBTxManager
		storageOperation            operationCreator
		notifierAPI                 mrnotifier.NoteProducer
		factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator
		factoryOperationDisable2FA  user2faOperationCreator
		errorWrapper                errors.Wrapper
	}
)

// NewDisable2FA - создаёт объект Disable2FA.
func NewDisable2FA(
	txManager mrstorage.DBTxManager,
	storageOperation operationCreator,
	notifierAPI mrnotifier.NoteProducer,
	factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator,
	factoryOperationDisable2FA user2faOperationCreator,
) *Disable2FA {
	return &Disable2FA{
		txManager:                   txManager,
		storageOperation:            storageOperation,
		notifierAPI:                 notifierAPI,
		factoryUser2FAConfirmAction: factoryUser2FAConfirmAction,
		factoryOperationDisable2FA:  factoryOperationDisable2FA,
		errorWrapper:                errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - создаёт операцию отключения 2FA и в той же транзакции отправляет
// пользователю код её подтверждения.
func (uc *Disable2FA) Execute(ctx context.Context, userID uuid.UUID) (secureoperation.SecureOperation, error) {
	if userID == uuid.Nil {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	user2FA, err := uc.factoryUser2FAConfirmAction.CreateByUserID(ctx, userID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	op, err := uc.factoryOperationDisable2FA.Create(user2FA) // проверяет, что 2FA включена
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
					"confirm.disable.2fa",
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
