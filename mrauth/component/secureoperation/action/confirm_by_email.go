package action

import (
	"errors"
	"time"

	"github.com/mondegor/go-sysmess/mrerr/mr"

	"github.com/mondegor/go-components/mrauth/bag/contactaddress"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/addresstype"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
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
func (a *ConfirmByEmail) Create(email contactaddress.ContactAddress, confirmCode string) (dto.ConfirmAction, error) {
	if email.Type != addresstype.Email {
		return dto.ConfirmAction{}, mr.ErrInternal.Wrap(errors.New("invalid contactAddress type")).WithAttr("email", email)
	}

	return dto.ConfirmAction{
		Method:        confirmmethod.Email,
		MaxAttempts:   a.maxAttempts,
		MaxResends:    a.maxResends,
		MinResendTime: a.minResendTime,
		Expiry:        a.expiry,
		Address:       email.Value,
		Secret:        confirmCode,
	}, nil
}
