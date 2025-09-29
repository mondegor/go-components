package action

import (
	"errors"
	"time"

	"github.com/mondegor/go-sysmess/mrerr/mr"

	"github.com/mondegor/go-components/mrauth/bag/contactaddress"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum"
)

type (
	// ConfirmByEmail - comment struct.
	ConfirmByEmail struct {
		maxAttempts   uint32
		maxResends    uint32
		minResendTime time.Duration
		expiry        time.Duration
	}
)

// NewConfirmByEmail - создаёт объект ConfirmByEmail.
func NewConfirmByEmail(opts ...Option) *ConfirmByEmail {
	co := newConfirmOptions(opts)

	return &ConfirmByEmail{
		maxAttempts:   co.maxAttempts,
		maxResends:    co.maxResends,
		minResendTime: co.minResendTime,
		expiry:        co.expiry,
	}
}

// Create - comments method.
func (a *ConfirmByEmail) Create(email contactaddress.ContactAddress, confirmCode string) (entity.ConfirmAction, error) {
	if email.Type != enum.AddressTypeEmail {
		return entity.ConfirmAction{}, mr.ErrInternal.Wrap(errors.New("invalid contactAddress type")).WithAttr("email", email)
	}

	return entity.ConfirmAction{
		Method:        enum.ConfirmMethodEmail,
		MaxAttempts:   a.maxAttempts,
		MaxResends:    a.maxResends,
		MinResendTime: a.minResendTime,
		Expiry:        a.expiry,
		Address:       email.Value,
		Secret:        confirmCode,
	}, nil
}
