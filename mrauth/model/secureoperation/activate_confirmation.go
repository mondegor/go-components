package secureoperation

import (
	"time"

	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
)

// ActivateConfirmation - comments method.
func (o *SecureOperation) ActivateConfirmation(token string) (err error) {
	if token == "" {
		return errors.ErrInternalIncorrectInputData.WithDetails("token is empty")
	}

	if o.Status != operationstatus.Opened || len(o.actions) == 0 {
		return ErrOperationAlreadyConfirmed // operation is not opened
	}

	action := &o.actions[0]

	if action.Sendable() {
		o.RemainingResends = action.MaxResends
		o.ResendsAt = time.Now().Add(action.MinResendTime).Round(1 * time.Second)
	}

	o.Token = token
	o.RemainingAttempts = action.MaxAttempts
	o.ExpiresAt = time.Now().Add(action.Expiry).Round(1 * time.Second)

	return nil
}
