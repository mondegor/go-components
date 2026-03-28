package secureoperation

import (
	"time"

	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
)

// ConfirmAction - comments method.
func (o *SecureOperation) ConfirmAction(checkCode func(method confirmmethod.Enum, code string) bool) (confirmed bool, err error) {
	// if item.Payload["audience"] == "" {
	// 	return 0, errors.New("invalid operation token")
	// }
	//
	// if item.Payload["visitor_id"] == "" {
	// 	return 0, errors.New("invalid operation token")
	// }
	if o.Status != operationstatus.Opened || len(o.actions) == 0 {
		return false, ErrOperationAlreadyConfirmed // operation is not opened
	}

	if o.RemainingAttempts <= 0 {
		return false, ErrNoAttemptsToConfirmOperation // :TODO: задокументировать возвращение operation
	}

	action := o.actions[0]

	if !action.Sendable() {
		return false, errors.New("confirming action not sendable")
	}

	if !checkCode(action.Method, action.ConfirmCode) {
		o.RemainingAttempts--

		return false, ErrConfirmCodeIsIncorrect
	}

	o.actions = o.actions[1:]

	// если следующие операции есть, то всё ок!
	if len(o.actions) > 0 {
		return false, nil
	}

	o.Status = operationstatus.Confirmed
	o.ExpiresAt = time.Now().Add(action.Expiry).Round(1 * time.Second)

	return true, nil
}
