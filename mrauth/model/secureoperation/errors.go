package secureoperation

import (
	"github.com/mondegor/go-core/errors"
)

var (
	// ErrOperationAlreadyExpired - operation already expired.
	ErrOperationAlreadyExpired = errors.NewUserError("OperationAlreadyExpired", "operation already expired")

	// ErrOperationIsNotConfirmed - operation is not confirmed.
	ErrOperationIsNotConfirmed = errors.NewUserError("OperationIsNotConfirmed", "operation is not confirmed")

	// ErrOperationAlreadyConfirmed - operation already confirmed.
	ErrOperationAlreadyConfirmed = errors.NewUserError("OperationAlreadyConfirmed", "operation already confirmed")

	// ErrSendingNewMessagesIsTemporarilyRestricted - sending new messages is temporarily restricted.
	ErrSendingNewMessagesIsTemporarilyRestricted = errors.NewUserError(
		"SendingNewMessagesIsTemporarilyRestricted", "sending new messages is temporarily restricted")

	// ErrConfirmCodeIsIncorrect - confirm code is incorrect.
	ErrConfirmCodeIsIncorrect = errors.NewUserError("ConfirmCodeIsIncorrect", "confirm code is incorrect")

	// ErrNoAttemptsToConfirmOperation - all attempts to confirm the operation have been spent.
	ErrNoAttemptsToConfirmOperation = errors.NewUserError("NoAttemptsToConfirmOperation", "all attempts to confirm the operation have been spent")
)
