package contactaddress

import (
	"github.com/mondegor/go-core/errors"
)

var (
	// ErrAddressIsInvalid - address is invalid.
	ErrAddressIsInvalid = errors.NewUserError("AddressIsInvalid", "address is invalid")

	// ErrEmailIsInvalid - email is invalid.
	ErrEmailIsInvalid = errors.NewUserError("EmailIsInvalid", "email is invalid")

	// ErrPhoneIsInvalid - phone is invalid.
	ErrPhoneIsInvalid = errors.NewUserError("PhoneIsInvalid", "phone is invalid")
)
