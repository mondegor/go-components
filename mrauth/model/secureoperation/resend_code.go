package secureoperation

import (
	"time"

	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
)

// ActivateResendCode - повторно активирует операцию под новый токен для отправки нового кода подтверждения.
func (o *SecureOperation) ActivateResendCode(token string) (err error) {
	if token == "" {
		return errors.ErrInternalIncorrectInputData.WithDetails("token is empty")
	}

	if o.Status != operationstatus.Opened {
		return ErrOperationAlreadyConfirmed
	}

	// запрещено инвариантом (см. checkInvariants): у Opened всегда есть хотя бы одно действие
	if len(o.actions) == 0 {
		return errors.ErrInternalIncorrectInputData.WithDetails("operation is opened, but len(actions) == 0")
	}

	action := &o.actions[0]

	// у не-sendable действия (2FA: TOTP/password) нет кода для повторной отправки. Ситуация
	// достижима клиентом: в цепочке email -> TOTP после подтверждения email текущим становится
	// 2FA-действие, и повторная отправка по нему уже неприменима
	if !action.Sendable() {
		return ErrResendCodeIsNotSupported
	}

	if o.RemainingResends == 0 {
		return ErrNoAttemptsToResendCode
	}

	if time.Now().UTC().Before(o.ResendsAt) {
		return ErrSendingNewMessagesIsTemporarilyRestricted
	}

	o.Token = token
	o.RemainingAttempts = action.MaxAttempts
	o.ExpiresAt = time.Now().UTC().Add(action.Expiry).Round(1 * time.Second)

	o.RemainingResends--
	o.ResendsAt = time.Now().UTC().Add(action.MinResendTime).Round(1 * time.Second)

	return nil
}
