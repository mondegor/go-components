package action

import (
	"time"

	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// ConfirmByPassword - фабрика действий подтверждения паролем.
	ConfirmByPassword struct {
		maxAttempts int16
		expiry      time.Duration
	}
)

// NewConfirmByPassword - создаёт объект ConfirmByPassword.
func NewConfirmByPassword(opts ...Option) *ConfirmByPassword {
	o := newConfirmOptions(opts)

	return &ConfirmByPassword{
		maxAttempts: o.maxAttempts,
		expiry:      o.expiry,
	}
}

// Create - создаёт действие подтверждения паролем.
func (a *ConfirmByPassword) Create(_ string) secureoperation.ConfirmAction {
	return secureoperation.ConfirmAction{
		Method:      confirmmethod.Password,
		MaxAttempts: a.maxAttempts,
		Expiry:      a.expiry,
		// ConfirmCode:     hashedPassword,
	}
}
