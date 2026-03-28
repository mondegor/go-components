package mrauth

import (
	"github.com/mondegor/go-sysmess/errors"
)

var (
	// ErrTokenInvalid - token is invalid.
	ErrTokenInvalid = errors.NewUserProto("TokenInvalid", "access token is invalid")

	// ErrTokenNotFoundOrExpired - token not found or expired.
	ErrTokenNotFoundOrExpired = errors.NewUserProto("TokenNotFoundOrExpired", "access token not found or expired")

	// // ErrTokenRejected - token was rejected.
	// ErrTokenRejected = errors.NewUserProto("ErrTokenRejected", "access token was rejected: '{Reason}'").

	// ErrLoginNotExists - login not exists.
	ErrLoginNotExists = errors.NewUserProto("ErrLoginNotExists", "login not exists")

	// ErrEmailAlreadyExists - entity already exists.
	ErrEmailAlreadyExists = errors.NewUserProto("EmailAlreadyExists", "email already exists")

	// ErrPhoneAlreadyExists - entity already exists.
	ErrPhoneAlreadyExists = errors.NewUserProto("PhoneAlreadyExists", "phone already exists")
)
