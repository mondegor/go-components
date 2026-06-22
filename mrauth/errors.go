package mrauth

import (
	"github.com/mondegor/go-sysmess/errors"
)

var (
	// ErrTokenInvalid - token is invalid.
	ErrTokenInvalid = errors.NewUserError("TokenInvalid", "auth token is invalid")

	// ErrTokenNotFoundOrExpired - token not found or expired.
	ErrTokenNotFoundOrExpired = errors.NewUserError("TokenNotFoundOrExpired", "auth token not found or expired")

	// ErrTooManyOpenSessionRequests - сработало временное ограничение на открытие новой сессии:
	// вход выполняется слишком часто, нужно повторить попытку позже.
	ErrTooManyOpenSessionRequests = errors.NewUserError("TooManyOpenSessionRequests", "too many session open requests, try again later")

	// ErrLoginNotExists - login not exists.
	ErrLoginNotExists = errors.NewUserError("ErrLoginNotExists", "login not exists")

	// ErrEmailAlreadyExists - entity already exists.
	ErrEmailAlreadyExists = errors.NewUserError("EmailAlreadyExists", "email already exists")

	// ErrPhoneAlreadyExists - entity already exists.
	ErrPhoneAlreadyExists = errors.NewUserError("PhoneAlreadyExists", "phone already exists")

	// Err2FAMustBeDisabledFirst - установка нового второго фактора (пароль/TOTP)
	// требует предварительного отключения текущего 2FA (нельзя менять активный фактор на месте).
	//nolint:errname // каноническое написание 2FA: errname не допускает цифру после Err.
	Err2FAMustBeDisabledFirst = errors.NewUserError("2FAMustBeDisabledFirst", "disable current 2fa before setting a new one")
)
