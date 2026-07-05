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
	// ConfirmByEmail - фабрика действий подтверждения по email.
	ConfirmByEmail struct {
		maxAttempts   int16
		maxResends    int16
		minResendTime time.Duration
		expiry        time.Duration
	}
)

// NewConfirmByEmail - создаёт объект ConfirmByEmail.
func NewConfirmByEmail(opts ...Option) *ConfirmByEmail {
	o := newConfirmOptions(opts)

	return &ConfirmByEmail{
		maxAttempts:   o.maxAttempts,
		maxResends:    o.maxResends,
		minResendTime: o.minResendTime,
		expiry:        o.expiry,
	}
}

// Create - создаёт действие подтверждения по email; в ConfirmCode сохраняется хеш
// кода (для хранения), в PlainConfirmCode - открытый код (для отправки пользователю).
func (a *ConfirmByEmail) Create(email contactaddress.ContactAddress, confirmCode, hashedConfirmCode string) (secureoperation.ConfirmAction, error) {
	if !email.Is(addresstype.Email) {
		return secureoperation.ConfirmAction{},
			errors.NewInternalError(
				"contactAddress type is invalid",
				"email", email,
			)
	}

	return secureoperation.ConfirmAction{
		Method:           confirmmethod.Email,
		MaxAttempts:      a.maxAttempts,
		MaxResends:       a.maxResends,
		MinResendTime:    a.minResendTime,
		Expiry:           a.expiry,
		Address:          email.Value(),
		ConfirmCode:      hashedConfirmCode,
		PlainConfirmCode: confirmCode,
	}, nil
}
