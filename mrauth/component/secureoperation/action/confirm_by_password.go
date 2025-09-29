package action

import (
	"time"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum"
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
	co := newConfirmOptions(opts)

	return &ConfirmByPassword{
		maxAttempts: co.maxAttempts,
		expiry:      co.expiry,
	}
}

// Create - comments method.
func (a *ConfirmByPassword) Create(hashedPassword string) entity.ConfirmAction {
	return entity.ConfirmAction{
		Method:      enum.ConfirmMethodPassword,
		MaxAttempts: a.maxAttempts,
		Expiry:      a.expiry,
		Secret:      hashedPassword,
	}
}
