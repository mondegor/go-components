package action

import (
	"errors"

	"github.com/mondegor/go-sysmess/mrerr/mr"

	"github.com/mondegor/go-components/mrauth/bag/contactaddress"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum"
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
func (a *ConfirmByAddress) Create(address contactaddress.ContactAddress, confirmCode string) (entity.ConfirmAction, error) {
	if address.Type == enum.AddressTypePhone {
		return a.confirmByPhone.Create(address, confirmCode)
	}

	if address.Type == enum.AddressTypeEmail {
		return a.confirmByEmail.Create(address, confirmCode)
	}

	return entity.ConfirmAction{}, mr.ErrInternal.Wrap(errors.New("invalid contactAddress type")).WithAttr("address", address)
}
