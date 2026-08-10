package unit

import (
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

// newConfirmActionBy2FA - строит действие подтверждения операции по описанию второго
// фактора пользователя. AllowRecovery здесь не выставляется: допустимость аварийного
// кода - решение конкретной операции, а не свойство фактора.
func newConfirmActionBy2FA(action2fa dto.ConfirmAction2FA) secureoperation.ConfirmAction {
	return secureoperation.ConfirmAction{
		Method:      action2fa.Method,
		MaxAttempts: action2fa.MaxAttempts,
		Expiry:      action2fa.Expiry,
	}
}
