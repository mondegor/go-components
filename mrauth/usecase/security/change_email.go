package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// ChangeEmailProperty - создаёт операцию смены email пользователя (с проверкой
	// доступности адреса) и отправляет код её подтверждения.
	ChangeEmailProperty struct {
		opener                      operationOpener
		emailChecker                userEmailChecker
		factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator
		factoryOperationEmail       factoryOperationValue2FA
		errorWrapper                errors.Wrapper
	}

	// operationOpener - открывает созданную операцию: гасит прежние операции того же
	// типа, сохраняет новую, отправляет код подтверждения и пишет журнал.
	operationOpener interface {
		Open(
			ctx context.Context,
			actor dto.ActorMeta,
			op secureoperation.SecureOperation,
			noteName string,
			noteProps conv.Group,
		) error
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
	opener operationOpener,
	emailChecker userEmailChecker,
	factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator,
	factoryOperationEmail factoryOperationValue2FA,
) *ChangeEmailProperty {
	return &ChangeEmailProperty{
		opener:                      opener,
		emailChecker:                emailChecker,
		factoryUser2FAConfirmAction: factoryUser2FAConfirmAction,
		factoryOperationEmail:       factoryOperationEmail,
		errorWrapper:                errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - проверяет доступность нового email, создаёт операцию его смены и в той
// же транзакции отправляет пользователю код её подтверждения.
func (uc *ChangeEmailProperty) Execute(
	ctx context.Context,
	actor dto.ActorMeta,
	newEmail string,
) (secureoperation.SecureOperation, error) {
	if actor.VisitorID == uuid.Nil {
		return secureoperation.SecureOperation{}, errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	parsedEmail, err := contactaddress.ParseEmail(newEmail)
	if err != nil {
		return secureoperation.SecureOperation{}, errors.ErrIncorrectInputData.New(err)
	}

	if err := uc.emailChecker.CheckAvailabilityEmail(ctx, parsedEmail); err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	user2FA, err := uc.factoryUser2FAConfirmAction.CreateByUserID(ctx, actor.VisitorID) // TODO: объединить CreateByUserLogin и CreateByUserID
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	op, err := uc.factoryOperationEmail.Create(user2FA, newEmail)
	if err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	if err = uc.opener.Open(ctx, actor, op, "confirm.change.email", nil); err != nil {
		return secureoperation.SecureOperation{}, uc.errorWrapper.Wrap(err)
	}

	return op, nil
}
