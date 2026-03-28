package secureoperation

import (
	"github.com/mondegor/go-sysmess/errors"
)

var (
	// ErrOperationAlreadyExpired - operation already expired.
	ErrOperationAlreadyExpired = errors.NewUserProto("OperationAlreadyExpired", "operation already expired")

	// ErrOperationIsNotConfirmed - operation is not confirmed.
	ErrOperationIsNotConfirmed = errors.NewUserProto("OperationIsNotConfirmed", "operation is not confirmed")

	// ErrOperationAlreadyConfirmed - operation already confirmed.
	ErrOperationAlreadyConfirmed = errors.NewUserProto("OperationAlreadyConfirmed", "operation already confirmed")

	// ErrSendingNewMessagesIsTemporarilyRestricted - sending new messages is temporarily restricted.
	ErrSendingNewMessagesIsTemporarilyRestricted = errors.NewUserProto(
		"SendingNewMessagesIsTemporarilyRestricted", "sending new messages is temporarily restricted")

	// ErrConfirmCodeIsIncorrect - confirm code is incorrect.
	ErrConfirmCodeIsIncorrect = errors.NewUserProto("ConfirmCodeIsIncorrect", "confirm code is incorrect")

	// ErrNoAttemptsToConfirmOperation - all attempts to confirm the operation have been spent.
	ErrNoAttemptsToConfirmOperation = errors.NewUserProto("NoAttemptsToConfirmOperation", "all attempts to confirm the operation have been spent")
)
