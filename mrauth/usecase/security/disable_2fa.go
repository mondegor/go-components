package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// Disable2FA - создаёт операцию отключения 2FA пользователя и отправляет код
	// её подтверждения.
	Disable2FA struct {
		opener                      operationOpener
		factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator
		factoryOperationDisable2FA  user2faOperationCreator
		errorWrapper                errors.Wrapper
	}
)

// NewDisable2FA - создаёт объект Disable2FA.
func NewDisable2FA(
	opener operationOpener,
	factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator,
	factoryOperationDisable2FA user2faOperationCreator,
) *Disable2FA {
	return &Disable2FA{
		opener:                      opener,
		factoryUser2FAConfirmAction: factoryUser2FAConfirmAction,
		factoryOperationDisable2FA:  factoryOperationDisable2FA,
		errorWrapper:                errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - создаёт операцию отключения 2FA и в той же транзакции отправляет
// пользователю код её подтверждения.
func (uc *Disable2FA) Execute(ctx context.Context, actor dto.ActorMeta) (secureoperation.SecureOperation, error) {
	if actor.VisitorID == uuid.Nil {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	user2FA, err := uc.factoryUser2FAConfirmAction.CreateByUserID(ctx, actor.VisitorID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	op, err := uc.factoryOperationDisable2FA.Create(user2FA) // проверяет, что 2FA включена
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	if err = uc.opener.Open(ctx, actor, op, "confirm.disable.2fa", nil); err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	return op, nil
}
