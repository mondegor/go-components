package action

import (
	"time"

	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth/enum/addresstype"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// ConfirmByPhone - comment struct.
	ConfirmByPhone struct {
		maxAttempts   int16
		maxResends    int16
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

// Create - создаёт действие подтверждения по телефону; в ConfirmCode сохраняется хеш
// кода (для хранения), в PlainConfirmCode - открытый код (для отправки пользователю).
func (a *ConfirmByPhone) Create(phone contactaddress.ContactAddress, confirmCode, hashedConfirmCode string) (secureoperation.ConfirmAction, error) {
	if !phone.Is(addresstype.Phone) {
		return secureoperation.ConfirmAction{},
			errors.NewInternalError(
				"contactAddress type is invalid",
				"phone", phone,
			)
	}

	return secureoperation.ConfirmAction{
		Method:           confirmmethod.Phone,
		MaxAttempts:      a.maxAttempts,
		MaxResends:       a.maxResends,
		MinResendTime:    a.minResendTime,
		Expiry:           a.expiry,
		Address:          phone.Value(),
		ConfirmCode:      hashedConfirmCode,
		PlainConfirmCode: confirmCode,
	}, nil
}
