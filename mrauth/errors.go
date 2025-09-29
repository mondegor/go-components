package mrauth

import "github.com/mondegor/go-sysmess/mrerr"

var (
	// ErrTokenInvalid - token is invalid.
	ErrTokenInvalid = mrerr.NewKindUser("TokenInvalid", "token is invalid")

	// ErrTokenNotFoundOrExpired - token not found or expired.
	ErrTokenNotFoundOrExpired = mrerr.NewKindUser("TokenNotFoundOrExpired", "token not found or expired")

	// // ErrTokenRejected - token was rejected.
	// ErrTokenRejected = mrerr.NewKindUser("ErrTokenRejected", "token was rejected: '{Reason}'").

	// ErrConfirmCodeIsIncorrect - confirm code is incorrect.
	ErrConfirmCodeIsIncorrect = mrerr.NewKindUser("ConfirmCodeIsIncorrect", "confirm code is incorrect")

	// ErrNoAttemptsToConfirmOperation - all attempts to confirm the operation have been used.
	ErrNoAttemptsToConfirmOperation = mrerr.NewKindUser("NoAttemptsToConfirmOperation", "all attempts to confirm the operation have been used")

	// ErrSendingNewMessagesIsTemporarilyRestricted - sending new messages is temporarily restricted.
	ErrSendingNewMessagesIsTemporarilyRestricted = mrerr.NewKindUser(
		"SendingNewMessagesIsTemporarilyRestricted", "sending new messages is temporarily restricted")

	// ErrOperationAlreadyExpired - operation already expired.
	ErrOperationAlreadyExpired = mrerr.NewKindUser("OperationAlreadyExpired", "operation already expired")

	// ErrOperationIsNotConfirmed - operation is not confirmed.
	ErrOperationIsNotConfirmed = mrerr.NewKindUser("OperationIsNotConfirmed", "operation is not confirmed")

	// ErrOperationAlreadyConfirmed - operation already confirmed.
	ErrOperationAlreadyConfirmed = mrerr.NewKindUser("OperationAlreadyConfirmed", "operation already confirmed")

	// ErrLoginNotExists - login not exists.
	ErrLoginNotExists = mrerr.NewKindUser("ErrLoginNotExists", "login not exists")

	// ErrEmailAlreadyExists - entity already exists.
	ErrEmailAlreadyExists = mrerr.NewKindUser("EmailAlreadyExists", "email already exists")

	// ErrPhoneAlreadyExists - entity already exists.
	ErrPhoneAlreadyExists = mrerr.NewKindUser("PhoneAlreadyExists", "phone already exists")
)
