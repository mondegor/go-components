package mrauth

import (
	"github.com/mondegor/go-core/errors"
)

var (
	// ErrTokenInvalid - token is invalid.
	ErrTokenInvalid = errors.NewUserError("TokenInvalid", "auth token is invalid")

	// ErrTokenNotFoundOrExpired - token not found or expired.
	ErrTokenNotFoundOrExpired = errors.NewUserError("TokenNotFoundOrExpired", "auth token not found or expired")

	// ErrSessionLimitExceededTryLater - превышен лимит одновременных сессий (hard-порог):
	// вход временно отклонён, пока фоновая чистка не освободит место. Нужно повторить попытку позже.
	ErrSessionLimitExceededTryLater = errors.NewUserError("SessionLimitExceededTryLater", "session limit exceeded, try again later")

	// ErrLoginNotExists - login not exists.
	ErrLoginNotExists = errors.NewUserError("ErrLoginNotExists", "login not exists")

	// ErrEmailAlreadyExists - entity already exists.
	ErrEmailAlreadyExists = errors.NewUserError("EmailAlreadyExists", "email already exists")

	// ErrSignupAlreadyInProgressTryLater - для этого email уже идёт процесс регистрации (код
	// недавно отправлен): повторная попытка временно отклонена как анти-спам. Не раскрывает,
	// зарегистрирован ли email. Нужно повторить попытку позже.
	ErrSignupAlreadyInProgressTryLater = errors.NewUserError("SignupAlreadyInProgressTryLater", "signup already in progress, try again later")

	// ErrPhoneAlreadyExists - entity already exists.
	ErrPhoneAlreadyExists = errors.NewUserError("PhoneAlreadyExists", "phone already exists")

	// Err2FAMustBeDisabledFirst - установка нового второго фактора (пароль/TOTP)
	// требует предварительного отключения текущего 2FA (нельзя менять активный фактор на месте).
	//nolint:errname // каноническое написание 2FA: errname не допускает цифру после Err.
	Err2FAMustBeDisabledFirst = errors.NewUserError("2FAMustBeDisabledFirst", "disable current 2fa before setting a new one")
)
