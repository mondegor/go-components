package secureoperation

import (
	"time"

	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
)

// ConfirmAction - проверяет текущее действие операции через checkFunc; при успехе
// переходит к следующему действию или переводит операцию в статус Confirmed.
func (o *SecureOperation) ConfirmAction(checkFunc func(action ConfirmAction) (ok bool, err error)) (confirmed bool, err error) {
	if o.Status != operationstatus.Opened {
		return false, ErrOperationAlreadyConfirmed // нет открытого действия для подтверждения
	}

	// запрещено инвариантом (см. checkInvariants): у Opened всегда есть хотя бы одно действие
	if len(o.actions) == 0 {
		return false, errors.ErrInternalIncorrectInputData.WithDetails("operation is opened, but len(actions) == 0")
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
	o.ExpiresAt = time.Now().UTC().Add(action.Expiry).Round(1 * time.Second)

	return true, nil
}
