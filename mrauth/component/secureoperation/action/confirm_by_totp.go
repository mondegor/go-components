package action

import (
	"time"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
)

type (
	// ConfirmByTOTP - comment struct.
	ConfirmByTOTP struct {
		maxAttempts uint32
		expiry      time.Duration
	}
)

// NewConfirmByTOTP - создаёт объект ConfirmByTOTP.
func NewConfirmByTOTP(opts ...Option) *ConfirmByTOTP {
	co := newConfirmOptions(opts)

	return &ConfirmByTOTP{
		maxAttempts: co.maxAttempts,
		expiry:      co.expiry,
	}
}

// Create - comments method.
func (a *ConfirmByTOTP) Create(secret string) dto.ConfirmAction {
	return dto.ConfirmAction{
		Method:      confirmmethod.TOTP,
		MaxAttempts: a.maxAttempts,
		Expiry:      a.expiry,
		Secret:      secret,
	}
}
