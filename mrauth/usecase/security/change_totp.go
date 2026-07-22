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
	// ChangeTOTPGeneratorProperty - создаёт операцию смены TOTP-генератора пользователя
	// и отправляет код её подтверждения.
	ChangeTOTPGeneratorProperty struct {
		opener                      operationOpener
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
	opener operationOpener,
	factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator,
	factoryOperationTOTP user2faOperationCreator,
) *ChangeTOTPGeneratorProperty {
	return &ChangeTOTPGeneratorProperty{
		opener:                      opener,
		factoryUser2FAConfirmAction: factoryUser2FAConfirmAction,
		factoryOperationTOTP:        factoryOperationTOTP,
		errorWrapper:                errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - создаёт операцию смены TOTP-генератора и в той же транзакции отправляет
// пользователю код её подтверждения.
func (uc *ChangeTOTPGeneratorProperty) Execute(ctx context.Context, actor dto.ActorMeta) (secureoperation.SecureOperation, error) {
	if actor.VisitorID == uuid.Nil {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	user2FA, err := uc.factoryUser2FAConfirmAction.CreateByUserID(ctx, actor.VisitorID) // TODO: объединить CreateByUserLogin и CreateByUserID
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

	if err = uc.opener.Open(ctx, actor, op, "confirm.change.totp", nil); err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	return op, nil
}
