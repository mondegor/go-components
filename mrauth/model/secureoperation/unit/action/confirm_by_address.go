package action

import (
	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth/enum/addresstype"
	"github.com/mondegor/go-components/mrauth/model/contactaddress"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// ConfirmByAddress - comment struct.
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

// Create - comments method.
func (a *ConfirmByAddress) Create(address contactaddress.ContactAddress, confirmCode string) (secureoperation.ConfirmAction, error) {
	if address.Is(addresstype.Phone) {
		return a.confirmByPhone.Create(address, confirmCode)
	}

	if address.Is(addresstype.Email) {
		return a.confirmByEmail.Create(address, confirmCode)
	}

	return secureoperation.ConfirmAction{},
		errors.NewInternalError(
			"contactAddress type is invalid",
			"address", address,
		)
}
