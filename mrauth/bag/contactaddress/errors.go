package contactaddress

import "github.com/mondegor/go-sysmess/mrerr"

var (
	// ErrLoginIsInvalid - login is invalid.
	ErrLoginIsInvalid = mrerr.NewKindUser("LoginIsInvalid", "login is invalid")

	// ErrEmailIsInvalid - login is invalid.
	ErrEmailIsInvalid = mrerr.NewKindUser("EmailIsInvalid", "email is invalid")

	// ErrPhoneIsInvalid - login is invalid.
	ErrPhoneIsInvalid = mrerr.NewKindUser("PhoneIsInvalid", "phone is invalid")
)
