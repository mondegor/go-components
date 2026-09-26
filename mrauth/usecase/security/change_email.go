package security

import (
	"context"

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
		flow                  changeEmailFlow
		factoryOperationEmail factoryOperationAddress2FA
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

	factoryOperationAddress2FA interface {
		Create(user2FA dto.User2FA, fieldValue contactaddress.ContactAddress) (secureoperation.SecureOperation, error)
	}
)

// NewChangeEmailProperty - создаёт объект ChangeEmailProperty.
func NewChangeEmailProperty(
	opener operationOpener,
	emailChecker userEmailChecker,
	factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator,
	factoryOperationEmail factoryOperationAddress2FA,
) *ChangeEmailProperty {
	return &ChangeEmailProperty{
		flow:                  newChangeEmailFlow(opener, emailChecker, factoryUser2FAConfirmAction, "confirm.change.email.request"),
		factoryOperationEmail: factoryOperationEmail,
	}
}

// Execute - проверяет доступность нового email, создаёт операцию его смены и в той
// же транзакции отправляет пользователю код её подтверждения.
func (uc *ChangeEmailProperty) Execute(
	ctx context.Context,
	actor dto.ActorMeta,
	newEmail contactaddress.ContactAddress,
) (secureoperation.SecureOperation, error) {
	return uc.flow.execute(ctx, actor, newEmail, uc.factoryOperationEmail.Create)
}
