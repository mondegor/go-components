package action

import (
	"time"

	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
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
func (a *ConfirmByPassword) Create(hashedPassword string) secureoperation.ConfirmAction {
	return secureoperation.ConfirmAction{
		Method:      confirmmethod.Password,
		MaxAttempts: a.maxAttempts,
		Expiry:      a.expiry,
		Secret:      hashedPassword,
	}
}
