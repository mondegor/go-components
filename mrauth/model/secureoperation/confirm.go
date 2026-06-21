package secureoperation

import (
	"time"

	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
)

// ConfirmAction - проверяет текущее действие операции через checkFunc; при успехе
// переходит к следующему действию или переводит операцию в статус Confirmed.
func (o *SecureOperation) ConfirmAction(checkFunc func(action ConfirmAction) (ok bool, err error)) (confirmed bool, err error) {
	// if item.Payload["audience"] == "" {
	// 	return 0, errors.New("invalid operation token")
	// }
	//
	// if item.Payload["visitor_id"] == "" {
	// 	return 0, errors.New("invalid operation token")
	// }
	if o.Status != operationstatus.Opened || len(o.actions) == 0 {
		return false, ErrOperationAlreadyConfirmed // нет открытого действия для подтверждения
	}

	if o.RemainingAttempts <= 0 {
		return false, ErrNoAttemptsToConfirmOperation
	}

	action := o.actions[0]

	ok, err := checkFunc(action)
	if err != nil {
		return false, err
	}

	if !ok {
		o.RemainingAttempts--

		return false, ErrConfirmCodeIsIncorrect
	}

	// переход к следующему подтверждению операции
	o.actions = o.actions[1:]

	if len(o.actions) > 0 {
		return false, nil // если необходимо следующее подтверждение, то завершаем без подтверждения
	}

	o.Status = operationstatus.Confirmed
	o.ExpiresAt = time.Now().Add(action.Expiry).Round(1 * time.Second)

	return true, nil
}
