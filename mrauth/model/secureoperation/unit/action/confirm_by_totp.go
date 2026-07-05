package action

import (
	"time"

	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
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

// Create - создаёт действие подтверждения через TOTP.
func (a *ConfirmByTOTP) Create(_ string) secureoperation.ConfirmAction {
	return secureoperation.ConfirmAction{
		Method:      confirmmethod.TOTP,
		MaxAttempts: a.maxAttempts,
		Expiry:      a.expiry,
		// ConfirmCode:     secret,
	}
}
