package mrauth

import (
	"github.com/mondegor/go-sysmess/errors"
)

var (
	// ErrTokenInvalid - token is invalid.
	ErrTokenInvalid = errors.NewUserError("TokenInvalid", "access token is invalid")

	// ErrTokenNotFoundOrExpired - token not found or expired.
	ErrTokenNotFoundOrExpired = errors.NewUserError("TokenNotFoundOrExpired", "access token not found or expired")

	// // ErrTokenRejected - token was rejected.
	// ErrTokenRejected = errors.NewUserProto("ErrTokenRejected", "access token was rejected: '{Reason}'").

	// ErrLoginNotExists - login not exists.
	ErrLoginNotExists = errors.NewUserError("ErrLoginNotExists", "login not exists")

	// ErrEmailAlreadyExists - entity already exists.
	ErrEmailAlreadyExists = errors.NewUserError("EmailAlreadyExists", "email already exists")

	// ErrPhoneAlreadyExists - entity already exists.
	ErrPhoneAlreadyExists = errors.NewUserError("PhoneAlreadyExists", "phone already exists")
)
