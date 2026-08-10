package security

import (
	"context"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// ChangeEmailByRecoveryProperty - создаёт операцию смены email пользователя, утратившего
	// доступ к текущему адресу (с проверкой доступности нового адреса): подтверждается вторым
	// фактором и аварийным кодом, письмо не отправляется.
	ChangeEmailByRecoveryProperty struct {
		flow                  changeEmailFlow
		factoryOperationEmail factoryOperationAddress2FA
	}
)

// NewChangeEmailByRecoveryProperty - создаёт объект ChangeEmailByRecoveryProperty.
func NewChangeEmailByRecoveryProperty(
	opener operationOpener,
	emailChecker userEmailChecker,
	factoryUser2FAConfirmAction mrauth.User2FAConfirmActionCreator,
	factoryOperationEmail factoryOperationAddress2FA,
) *ChangeEmailByRecoveryProperty {
	return &ChangeEmailByRecoveryProperty{
		flow: newChangeEmailFlow(
			opener,
			emailChecker,
			factoryUser2FAConfirmAction,
			// шаблона нет: первое звено этой цепочки - второй фактор, кода к отправке
			// не возникает вовсе (см. unit.ChangeEmailByRecovery)
			"",
		),
		factoryOperationEmail: factoryOperationEmail,
	}
}

// Execute - создаёт операцию смены email для пользователя, утратившего доступ к текущему
// адресу: подтверждается вторым фактором и аварийным кодом, письмо не отправляется.
// При выключенной 2FA операция не создаётся (mrauth.ErrAuth2FAIsDisabled): доказательство
// у такого аккаунта одно - код на текущий адрес. Скрывать это не от кого, метод авторизованный.
func (uc *ChangeEmailByRecoveryProperty) Execute(
	ctx context.Context,
	actor dto.ActorMeta,
	newEmail contactaddress.ContactAddress,
) (secureoperation.SecureOperation, error) {
	return uc.flow.execute(ctx, actor, newEmail, uc.factoryOperationEmail.Create)
}
