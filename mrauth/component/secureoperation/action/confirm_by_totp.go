package action

import (
	"time"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum"
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
func (a *ConfirmByTOTP) Create(secret string) entity.ConfirmAction {
	return entity.ConfirmAction{
		Method:      enum.ConfirmMethodTOTP,
		MaxAttempts: a.maxAttempts,
		Expiry:      a.expiry,
		Secret:      secret,
	}
}
