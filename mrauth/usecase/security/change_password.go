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
	// ChangePasswordProperty - создаёт операцию смены пароля пользователя и отправляет
	// код её подтверждения.
	ChangePasswordProperty struct {
		opener                      operationOpener
		factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator
		factoryOperationPassword    factoryOperationSecret2FA
		errorWrapper                errors.Wrapper
	}

	factoryOperationSecret2FA interface {
		Create(user2FA dto.User2FA, secret string) (secureoperation.SecureOperation, error)
	}
)

// NewChangePasswordProperty - создаёт объект ChangePasswordProperty.
func NewChangePasswordProperty(
	opener operationOpener,
	factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator,
	factoryOperationPassword factoryOperationSecret2FA,
) *ChangePasswordProperty {
	return &ChangePasswordProperty{
		opener:                      opener,
		factoryUser2FAConfirmAction: factoryUser2FAConfirmAction,
		factoryOperationPassword:    factoryOperationPassword,
		errorWrapper:                errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - создаёт операцию смены пароля и в той же транзакции отправляет
// пользователю код её подтверждения.
func (uc *ChangePasswordProperty) Execute(
	ctx context.Context,
	actor dto.ActorMeta,
	newPassword string,
) (secureoperation.SecureOperation, error) {
	if actor.VisitorID == uuid.Nil {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	if newPassword == "" {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("newPassword is empty")
	}

	user2FA, err := uc.factoryUser2FAConfirmAction.CreateByUserID(ctx, actor.VisitorID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	// активный 2FA нельзя менять на месте: сначала нужно отключить текущий (disable 2FA)
	if user2FA.Action2FA.Method > 0 {
		return secureoperation.SecureOperation{}, mrauth.ErrAuth2FAMustBeDisabledFirst
	}

	op, err := uc.factoryOperationPassword.Create(user2FA, newPassword)
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	if err = uc.opener.Open(ctx, actor, op, "confirm.change.password", nil); err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	return op, nil
}
