package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// ChangePhoneProperty - создаёт операцию смены телефона пользователя (с проверкой
	// доступности номера) и отправляет код её подтверждения.
	ChangePhoneProperty struct {
		opener                      operationOpener
		phoneChecker                userPhoneChecker
		factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator
		factoryOperationPhone       factoryOperationAddress2FA
		errorWrapper                errors.Wrapper
	}

	userPhoneChecker interface {
		CheckAvailabilityPhone(ctx context.Context, userPhone contactaddress.ContactAddress) error
	}
)

// NewChangePhoneProperty - создаёт объект ChangePhoneProperty.
func NewChangePhoneProperty(
	opener operationOpener,
	phoneChecker userPhoneChecker,
	factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator,
	factoryOperationPhone factoryOperationAddress2FA,
) *ChangePhoneProperty {
	return &ChangePhoneProperty{
		opener:                      opener,
		phoneChecker:                phoneChecker,
		factoryUser2FAConfirmAction: factoryUser2FAConfirmAction,
		factoryOperationPhone:       factoryOperationPhone,
		errorWrapper:                errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - проверяет доступность нового телефона, создаёт операцию его смены и в той
// же транзакции отправляет пользователю код её подтверждения.
func (uc *ChangePhoneProperty) Execute(
	ctx context.Context,
	actor dto.ActorMeta,
	newPhone contactaddress.ContactAddress,
) (secureoperation.SecureOperation, error) {
	if actor.VisitorID == uuid.Nil {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	if newPhone.Value() == "" {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("newPhone is empty")
	}

	if err := uc.phoneChecker.CheckAvailabilityPhone(ctx, newPhone); err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	user2FA, err := uc.factoryUser2FAConfirmAction.CreateByUserID(ctx, actor.VisitorID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	op, err := uc.factoryOperationPhone.Create(user2FA, newPhone)
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	if err = uc.opener.Open(ctx, actor, op, "confirm.change.phone", nil); err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	return op, nil
}
