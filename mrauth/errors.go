package mrauth

import (
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
)

var (
	// ErrTokenInvalid - token is invalid.
	ErrTokenInvalid = errors.NewUserError("TokenInvalid", "auth token is empty or invalid")

	// ErrTokenNotFoundOrExpired - token not found or expired.
	ErrTokenNotFoundOrExpired = errors.NewUserError("TokenNotFoundOrExpired", "auth token not found or expired")

	// ErrSessionLimitExceededTryLater - превышен лимит одновременных сессий (hard-порог):
	// вход временно отклонён, пока фоновая чистка не освободит место. Нужно повторить попытку позже.
	ErrSessionLimitExceededTryLater = errors.NewUserError("SessionLimitExceededTryLater", "session limit exceeded, try again later")

	// ErrLoginNotExists - login not exists.
	ErrLoginNotExists = errors.NewUserError("LoginNotExists", "login not exists")

	// ErrPasswordIsTooWeak - надёжность нового пароля ниже порога, заданного приложением.
	ErrPasswordIsTooWeak = errors.NewUserError("PasswordIsTooWeak", "password is too weak")

	// ErrPasswordHasRecoveryCodeFormat - новый пароль имеет формат аварийного кода: на звене
	// пароля такой ввод неотличим от аварийного кода, который там не принимается.
	ErrPasswordHasRecoveryCodeFormat = errors.NewUserError("PasswordHasRecoveryCodeFormat", "password has recovery code format")

	// ErrEmailAlreadyExists - entity already exists.
	ErrEmailAlreadyExists = errors.NewUserError("EmailAlreadyExists", "email already exists")

	// ErrSignupAlreadyInProgressTryLater - для этого email уже идёт процесс регистрации (код
	// недавно отправлен): повторная попытка временно отклонена как анти-спам. Не раскрывает,
	// зарегистрирован ли email. Нужно повторить попытку позже.
	ErrSignupAlreadyInProgressTryLater = errors.NewUserError("SignupAlreadyInProgressTryLater", "signup already in progress, try again later")

	// ErrPhoneAlreadyExists - entity already exists.
	ErrPhoneAlreadyExists = errors.NewUserError("PhoneAlreadyExists", "phone already exists")

	// ErrAuth2FAMustBeDisabledFirst - установка нового второго фактора (пароль/TOTP)
	// требует предварительного отключения текущего 2FA (нельзя менять активный фактор на месте).
	ErrAuth2FAMustBeDisabledFirst = errors.NewUserError("Auth2FAMustBeDisabledFirst", "disable current 2fa before setting a new one")

	// ErrRecoveryCodeNotAllowed - аварийный код предъявлен на звене, где он не принимается
	// (код с емаила, пароль или TOTP-код без допуска аварийного). Попытка подтверждения
	// не расходуется, аварийный код не гасится.
	ErrRecoveryCodeNotAllowed = errors.NewUserError("RecoveryCodeNotAllowed", "recovery code is not accepted at this step")

	// ErrAuth2FAIsDisabled - действие требует включённой 2FA, а она выключена.
	ErrAuth2FAIsDisabled = errors.NewUserError("Auth2FAIsDisabled", "2fa is disabled")

	// ErrOperationInvalid - operation is invalid.
	ErrOperationInvalid = errors.NewUserError("OperationInvalid", "operation is empty or invalid")

	// ErrOperationAlreadyExpired - operation already expired.
	ErrOperationAlreadyExpired = errors.NewUserError("OperationAlreadyExpired", "operation already expired")

	// ErrOperationIsNotConfirmed - operation is not confirmed.
	ErrOperationIsNotConfirmed = errors.NewUserError("OperationIsNotConfirmed", "operation is not confirmed")

	// ErrOperationAlreadyConfirmed - operation already confirmed.
	ErrOperationAlreadyConfirmed = errors.NewUserError("OperationAlreadyConfirmed", "operation already confirmed")

	// ErrSendingNewMessagesIsTemporarilyRestricted - sending new messages is temporarily restricted.
	ErrSendingNewMessagesIsTemporarilyRestricted = errors.NewUserError(
		"SendingNewMessagesIsTemporarilyRestricted", "sending new messages is temporarily restricted")

	// ErrConfirmCodeIsRequired - подтверждать нечем: секрет не передан, а звено операции ещё открыто.
	ErrConfirmCodeIsRequired = errors.NewUserError("ConfirmCodeIsRequired", "confirm code is required")

	// ErrConfirmCodeIsIncorrect - confirm code is incorrect.
	ErrConfirmCodeIsIncorrect = errors.NewUserError("ConfirmCodeIsIncorrect", "confirm code is incorrect")

	// ErrTOTPCodeIsIncorrect - введённый TOTP-код не совпал с кодом, ожидаемым для
	// секрета операции (или его time-step уже был использован).
	ErrTOTPCodeIsIncorrect = errors.NewUserError("TOTPCodeIsIncorrect", "totp code is incorrect")

	// ErrNoAttemptsToConfirmOperation - all attempts to confirm the operation have been spent.
	ErrNoAttemptsToConfirmOperation = errors.NewUserError("NoAttemptsToConfirmOperation", "all attempts to confirm the operation have been spent")

	// ErrResendCodeIsNotSupported - текущее действие операции не поддерживает
	// повторную отправку кода (2FA: TOTP/password).
	ErrResendCodeIsNotSupported = errors.NewUserError(
		"ResendCodeIsNotSupported", "resend confirm code is not supported for the current action")

	// ErrNoAttemptsToResendCode - все повторные отправки кода израсходованы. В отличие от
	// ErrSendingNewMessagesIsTemporarilyRestricted («ещё рано») это окончательный отказ:
	// ждать бессмысленно, операцию нужно создавать заново.
	ErrNoAttemptsToResendCode = errors.NewUserError("NoAttemptsToResendCode", "no attempts to resend confirm code")

	// ErrEventTokenExpired - предъявленный токен найден (или разобран), но срок его действия истёк.
	ErrEventTokenExpired = errors.New("token is expired")

	// ErrEventAuth2FACodeAlreadyUsed - предъявленный второй фактор (TOTP time-step или аварийный
	// код) уже израсходован конкурентным подтверждением.
	ErrEventAuth2FACodeAlreadyUsed = errors.New("2fa code is already used")
)

type (
	// RetryAfterError - причина временного отказа, несущая срок, через который запрос
	// имеет смысл повторить (заголовок Retry-After ответа 429).
	RetryAfterError struct {
		RetryAfter time.Duration
	}
)

// NewRetryAfterError - создаёт причину временного отказа с указанным сроком повторной попытки.
func NewRetryAfterError(retryAfter time.Duration) *RetryAfterError {
	return &RetryAfterError{
		RetryAfter: retryAfter,
	}
}

func (e *RetryAfterError) Error() string {
	return "retry after " + e.RetryAfter.String()
}

type (
	// TokenAlreadyRevokedError - ошибка, когда токен уже отозван.
	TokenAlreadyRevokedError struct {
		UserID    uuid.UUID
		SessionID uint32
	}
)

// NewTokenAlreadyRevokedError - создаёт ошибку TokenAlreadyRevokedError для указанного типа параметра.
func NewTokenAlreadyRevokedError(userID uuid.UUID, sessionID uint32) *TokenAlreadyRevokedError {
	return &TokenAlreadyRevokedError{
		UserID:    userID,
		SessionID: sessionID,
	}
}

func (e *TokenAlreadyRevokedError) Error() string {
	return "token is already revoked: " + e.UserID.String()
}
