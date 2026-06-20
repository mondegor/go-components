package mrauth

import (
	"github.com/mondegor/go-sysmess/errors"
)

var (
	// ErrTokenInvalid - token is invalid.
	ErrTokenInvalid = errors.NewUserError("TokenInvalid", "auth token is invalid")

	// ErrTokenNotFoundOrExpired - token not found or expired.
	ErrTokenNotFoundOrExpired = errors.NewUserError("TokenNotFoundOrExpired", "auth token not found or expired")

	// // ErrTokenRejected - token was rejected.
	// ErrTokenRejected = errors.NewUserProto("ErrTokenRejected", "access token was rejected: '{Reason}'").

	// ErrTooManyOpenSessionRequests - сработало временное ограничение на открытие новой сессии:
	// вход выполняется слишком часто, нужно повторить попытку позже.
	ErrTooManyOpenSessionRequests = errors.NewUserError("TooManyOpenSessionRequests", "too many session open requests, try again later")

	// ErrLoginNotExists - login not exists.
	ErrLoginNotExists = errors.NewUserError("ErrLoginNotExists", "login not exists")

	// ErrEmailAlreadyExists - entity already exists.
	ErrEmailAlreadyExists = errors.NewUserError("EmailAlreadyExists", "email already exists")

	// ErrPhoneAlreadyExists - entity already exists.
	ErrPhoneAlreadyExists = errors.NewUserError("PhoneAlreadyExists", "phone already exists")
)
