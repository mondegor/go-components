package contactaddress

import "errors"

var (
	// ErrAddressIsInvalid - address is invalid.
	ErrAddressIsInvalid = errors.New("address is invalid")

	// ErrEmailIsInvalid - email is invalid.
	ErrEmailIsInvalid = errors.New("email is invalid")

	// ErrPhoneIsInvalid - phone is invalid.
	ErrPhoneIsInvalid = errors.New("phone is invalid")
)
