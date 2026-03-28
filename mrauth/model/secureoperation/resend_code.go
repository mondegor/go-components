package secureoperation

import (
	"time"

	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
)

// ActivateResendCode - comments method.
func (o *SecureOperation) ActivateResendCode(token string) (err error) {
	if token == "" {
		return errors.ErrInternalIncorrectInputData.WithDetails("token is empty")
	}

	if o.Status != operationstatus.Opened || len(o.actions) == 0 {
		return ErrOperationAlreadyConfirmed // operation is not opened
	}

	action := &o.actions[0]

	if !action.Sendable() {
		return errors.New("action not support resend")
	}

	if o.RemainingResends == 0 {
		return errors.New("operation failed resends")
	}

	if time.Now().Before(o.ResendsAt) {
		return ErrSendingNewMessagesIsTemporarilyRestricted // WARNING: 'op' используется с этой ошибкой
	}

	o.Token = token
	o.RemainingAttempts = action.MaxAttempts
	o.ExpiresAt = time.Now().Add(action.Expiry).Round(1 * time.Second)

	o.RemainingResends--
	o.ResendsAt = time.Now().Add(action.MinResendTime).Round(1 * time.Second)

	return nil
}
