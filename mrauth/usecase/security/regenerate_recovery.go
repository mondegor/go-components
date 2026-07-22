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
	// RegenerateRecoveryProperty - создаёт операцию перевыпуска аварийных кодов пользователя
	// и отправляет код её подтверждения.
	RegenerateRecoveryProperty struct {
		opener                      operationOpener
		factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator
		factoryOperationRegenerate  user2faOperationCreator
		errorWrapper                errors.Wrapper
	}
)

// NewRegenerateRecoveryProperty - создаёт объект RegenerateRecoveryProperty.
func NewRegenerateRecoveryProperty(
	opener operationOpener,
	factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator,
	factoryOperationRegenerate user2faOperationCreator,
) *RegenerateRecoveryProperty {
	return &RegenerateRecoveryProperty{
		opener:                      opener,
		factoryUser2FAConfirmAction: factoryUser2FAConfirmAction,
		factoryOperationRegenerate:  factoryOperationRegenerate,
		errorWrapper:                errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - создаёт операцию перевыпуска аварийных кодов и в той же транзакции
// отправляет пользователю код её подтверждения.
func (uc *RegenerateRecoveryProperty) Execute(
	ctx context.Context,
	actor dto.ActorMeta,
) (secureoperation.SecureOperation, error) {
	if actor.VisitorID == uuid.Nil {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	user2FA, err := uc.factoryUser2FAConfirmAction.CreateByUserID(ctx, actor.VisitorID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	op, err := uc.factoryOperationRegenerate.Create(user2FA) // проверяет, что 2FA включена
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	if err = uc.opener.Open(ctx, actor, op, "confirm.regenerate.recovery", nil); err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	return op, nil
}
