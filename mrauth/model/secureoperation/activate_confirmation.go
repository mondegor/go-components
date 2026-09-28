package secureoperation

import (
	"time"

	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
)

const (
	// fixedExpiryThreshold - порог продления срока жизни действия. Короткий срок (не больше
	// порога) продлевается на каждом шаге - активации, повторной отправке кода, подтверждении, -
	// чтобы пользователь гарантированно успел завершить операцию, даже если цепочка длинная.
	// Срок больше порога выставляется осознанно, с запасом на завершение операции, поэтому
	// назначается один раз, при её создании, и дальше не продлевается (подтверждённая операция
	// живёт до того же срока).
	fixedExpiryThreshold = 30 * time.Minute
)

// ActivateConfirmation - активирует подтверждение операции под указанный токен.
func (o *SecureOperation) ActivateConfirmation(token string) (err error) {
	if token == "" {
		return errors.ErrInternalIncorrectInputData.WithDetails("token is empty")
	}

	if o.Status != operationstatus.Opened {
		return mrauth.ErrOperationAlreadyConfirmed
	}

	// запрещено инвариантом (см. checkInvariants): у Opened всегда есть хотя бы одно действие
	if len(o.actions) == 0 {
		return errors.ErrInternalIncorrectInputData.WithDetails("operation is opened, but len(actions) == 0")
	}

	action := &o.actions[0]

	if action.Sendable() {
		o.RemainingResends = action.MaxResends
		o.ResendsAt = time.Now().UTC().Add(action.MinResendTime).Round(1 * time.Second)
	}

	o.Token = token
	o.RemainingAttempts = action.MaxAttempts
	o.renewExpiry(action)

	return nil
}

// renewExpiry - назначает операции срок действия по указанному действию; фиксированный срок
// (больше fixedExpiryThreshold) назначается только однажды, пока он ещё не задан.
func (o *SecureOperation) renewExpiry(action *ConfirmAction) {
	if action.Expiry > fixedExpiryThreshold && !o.ExpiresAt.IsZero() {
		return
	}

	o.ExpiresAt = time.Now().UTC().Add(action.Expiry).Round(1 * time.Second)
}
