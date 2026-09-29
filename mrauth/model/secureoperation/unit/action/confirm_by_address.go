package action

import (
	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth/enum/addresstype"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// ConfirmByAddress - создаёт действие подтверждения по контактному адресу (email или телефон).
	ConfirmByAddress struct {
		confirmByEmail *ConfirmByEmail
		confirmByPhone *ConfirmByPhone
	}
)

// NewConfirmByAddress - создаёт объект ConfirmByAddress.
func NewConfirmByAddress(emailOpts, phoneOpts []Option) *ConfirmByAddress {
	return &ConfirmByAddress{
		confirmByEmail: NewConfirmByEmail(emailOpts...),
		confirmByPhone: NewConfirmByPhone(phoneOpts...),
	}
}

// Create - создаёт действие подтверждения по контактному адресу (параметры канала
// выбираются по типу адреса).
func (a *ConfirmByAddress) Create(address contactaddress.ContactAddress) (secureoperation.ConfirmAction, error) {
	if address.Is(addresstype.Phone) {
		return a.confirmByPhone.Create(address)
	}

	if address.Is(addresstype.Email) {
		return a.confirmByEmail.Create(address)
	}

	return secureoperation.ConfirmAction{},
		errors.NewInternalError(
			"contactAddress type is invalid",
			"address", address,
		)
}
