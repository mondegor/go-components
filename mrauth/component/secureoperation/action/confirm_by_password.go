package action

import (
	"time"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
)

type (
	// ConfirmByPassword - comment struct.
	ConfirmByPassword struct {
		maxAttempts uint32
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

// Create - comments method.
func (a *ConfirmByPassword) Create(hashedPassword string) dto.ConfirmAction {
	return dto.ConfirmAction{
		Method:      confirmmethod.Password,
		MaxAttempts: a.maxAttempts,
		Expiry:      a.expiry,
		Secret:      hashedPassword,
	}
}
