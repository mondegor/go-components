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
	// ConfirmByEmail - фабрика действий подтверждения по email.
	ConfirmByEmail struct {
		codeLength    int16
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
		codeLength:    o.codeLength,
		maxAttempts:   o.maxAttempts,
		maxResends:    o.maxResends,
		minResendTime: o.minResendTime,
		expiry:        o.expiry,
	}
}

// Expiry - возвращает срок жизни создаваемого действия подтверждения.
func (a *ConfirmByEmail) Expiry() time.Duration {
	return a.expiry
}

// Create - создаёт действие подтверждения по email; сам код выпускается позже
// (SecureOperation.InitSendableAction) по длине, записанной в действие.
func (a *ConfirmByEmail) Create(email contactaddress.ContactAddress) (secureoperation.ConfirmAction, error) {
	if !email.Is(addresstype.Email) {
		return secureoperation.ConfirmAction{},
			errors.NewInternalError(
				"contactAddress type is invalid",
				"email", email,
			)
	}

	return secureoperation.ConfirmAction{
		Method:        confirmmethod.Email,
		MaxAttempts:   a.maxAttempts,
		MaxResends:    a.maxResends,
		MinResendTime: a.minResendTime,
		CodeLength:    a.codeLength,
		Expiry:        a.expiry,
		Address:       email.Value(),
	}, nil
}
