package action

import (
	"time"

	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth/enum/addresstype"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// ConfirmByPhone - фабрика действий подтверждения по телефону.
	ConfirmByPhone struct {
		codeLength    int16
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
		codeLength:    o.codeLength,
		maxAttempts:   o.maxAttempts,
		maxResends:    o.maxResends,
		minResendTime: o.minResendTime,
		expiry:        o.expiry,
	}
}

// Create - создаёт действие подтверждения по телефону; сам код выпускается позже
// (SecureOperation.InitSendableAction) по длине, записанной в действие.
func (a *ConfirmByPhone) Create(phone contactaddress.ContactAddress) (secureoperation.ConfirmAction, error) {
	if !phone.Is(addresstype.Phone) {
		return secureoperation.ConfirmAction{},
			errors.NewInternalError(
				"contactAddress type is invalid",
				"phone", phone,
			)
	}

	return secureoperation.ConfirmAction{
		Method:        confirmmethod.Phone,
		MaxAttempts:   a.maxAttempts,
		MaxResends:    a.maxResends,
		MinResendTime: a.minResendTime,
		CodeLength:    a.codeLength,
		Expiry:        a.expiry,
		Address:       phone.Value(),
	}, nil
}
