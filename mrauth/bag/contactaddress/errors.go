package contactaddress

import (
	"github.com/mondegor/go-sysmess/errors"
)

var (
	// ErrLoginIsInvalid - login is invalid.
	ErrLoginIsInvalid = errors.NewUserProto("LoginIsInvalid", "login is invalid")

	// ErrEmailIsInvalid - login is invalid.
	ErrEmailIsInvalid = errors.NewUserProto("EmailIsInvalid", "email is invalid")

	// ErrPhoneIsInvalid - login is invalid.
	ErrPhoneIsInvalid = errors.NewUserProto("PhoneIsInvalid", "phone is invalid")
)
