package action

import (
	"time"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
)

type (
	// ConfirmByTOTP - фабрика действий подтверждения через TOTP.
	ConfirmByTOTP struct {
		maxAttempts int16
		expiry      time.Duration
	}
)

// NewConfirmByTOTP - создаёт объект ConfirmByTOTP.
func NewConfirmByTOTP(opts ...Option) *ConfirmByTOTP {
	o := newConfirmOptions(opts)

	return &ConfirmByTOTP{
		maxAttempts: o.maxAttempts,
		expiry:      o.expiry,
	}
}

// Create - создаёт описание второго фактора "TOTP".
func (a *ConfirmByTOTP) Create() dto.ConfirmAction2FA {
	return dto.ConfirmAction2FA{
		Method:      confirmmethod.TOTP,
		MaxAttempts: a.maxAttempts,
		Expiry:      a.expiry,
	}
}
