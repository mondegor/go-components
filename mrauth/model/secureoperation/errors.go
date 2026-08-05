package secureoperation

import (
	"github.com/mondegor/go-core/errors"
)

var (
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
)
