package action

import (
	"time"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
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

// Create - создаёт описание второго фактора "пароль".
func (a *ConfirmByPassword) Create() dto.ConfirmAction2FA {
	return dto.ConfirmAction2FA{
		Method:      confirmmethod.Password,
		MaxAttempts: a.maxAttempts,
		Expiry:      a.expiry,
	}
}
