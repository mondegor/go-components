package contactaddress

import (
	"github.com/mondegor/go-sysmess/errors"
)

var (
	// ErrAddressIsInvalid - address is invalid.
	ErrAddressIsInvalid = errors.NewUserProto("AddressIsInvalid", "address is invalid")

	// ErrEmailIsInvalid - email is invalid.
	ErrEmailIsInvalid = errors.NewUserProto("EmailIsInvalid", "email is invalid")

	// ErrPhoneIsInvalid - phone is invalid.
	ErrPhoneIsInvalid = errors.NewUserProto("PhoneIsInvalid", "phone is invalid")
)
