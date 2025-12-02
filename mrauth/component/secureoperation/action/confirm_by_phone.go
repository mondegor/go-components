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
	co := newConfirmOptions(opts)

	return &ConfirmByPhone{
		maxAttempts:   co.maxAttempts,
		maxResends:    co.maxResends,
		minResendTime: co.minResendTime,
		expiry:        co.expiry,
	}
}

// Create - comments method.
func (a *ConfirmByPhone) Create(phone contactaddress.ContactAddress, confirmCode string) (dto.ConfirmAction, error) {
	if phone.Type != addresstype.Phone {
		return dto.ConfirmAction{}, mr.ErrInternal.Wrap(errors.New("invalid contactAddress type")).WithAttr("phone", phone)
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
