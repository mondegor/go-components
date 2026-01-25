package action

import (
	"time"

	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth/bag/contactaddress"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/addresstype"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
)

type (
	// ConfirmByPhone - comment struct.
	ConfirmByPhone struct {
		maxAttempts   uint32
		maxResends    uint32
		minResendTime time.Duration
		expiry        time.Duration
	}
)

// NewConfirmByPhone - создаёт объект ConfirmByPhone.
func NewConfirmByPhone(opts ...Option) *ConfirmByPhone {
	o := newConfirmOptions(opts)

	return &ConfirmByPhone{
		maxAttempts:   o.maxAttempts,
		maxResends:    o.maxResends,
		minResendTime: o.minResendTime,
		expiry:        o.expiry,
	}
}

// Create - comments method.
func (a *ConfirmByPhone) Create(phone contactaddress.ContactAddress, confirmCode string) (dto.ConfirmAction, error) {
	if phone.Type != addresstype.Phone {
		return dto.ConfirmAction{},
			errors.NewInternalError(
				"contactAddress type is invalid",
				"phone", phone,
			)
	}

	return dto.ConfirmAction{
		Method:        confirmmethod.Phone,
		MaxAttempts:   a.maxAttempts,
		MaxResends:    a.maxResends,
		MinResendTime: a.minResendTime,
		Expiry:        a.expiry,
		Address:       phone.Value,
		Secret:        confirmCode,
	}, nil
}
