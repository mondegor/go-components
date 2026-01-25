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

	// ErrConfirmCodeIsIncorrect - confirm code is incorrect.
	ErrConfirmCodeIsIncorrect = errors.NewUserProto("ConfirmCodeIsIncorrect", "confirm code is incorrect")

	// ErrNoAttemptsToConfirmOperation - all attempts to confirm the operation have been used.
	ErrNoAttemptsToConfirmOperation = errors.NewUserProto("NoAttemptsToConfirmOperation", "all attempts to confirm the operation have been used")

	// ErrSendingNewMessagesIsTemporarilyRestricted - sending new messages is temporarily restricted.
	ErrSendingNewMessagesIsTemporarilyRestricted = errors.NewUserProto(
		"SendingNewMessagesIsTemporarilyRestricted", "sending new messages is temporarily restricted")

	// ErrOperationAlreadyExpired - operation already expired.
	ErrOperationAlreadyExpired = errors.NewUserProto("OperationAlreadyExpired", "operation already expired")

	// ErrOperationIsNotConfirmed - operation is not confirmed.
	ErrOperationIsNotConfirmed = errors.NewUserProto("OperationIsNotConfirmed", "operation is not confirmed")

	// ErrOperationAlreadyConfirmed - operation already confirmed.
	ErrOperationAlreadyConfirmed = errors.NewUserProto("OperationAlreadyConfirmed", "operation already confirmed")

	// ErrLoginNotExists - login not exists.
	ErrLoginNotExists = errors.NewUserProto("ErrLoginNotExists", "login not exists")

	// ErrEmailAlreadyExists - entity already exists.
	ErrEmailAlreadyExists = errors.NewUserProto("EmailAlreadyExists", "email already exists")

	// ErrPhoneAlreadyExists - entity already exists.
	ErrPhoneAlreadyExists = errors.NewUserProto("PhoneAlreadyExists", "phone already exists")
)
