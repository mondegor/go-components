package action

import (
	"time"

	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// ConfirmByRecovery - фабрика действий подтверждения аварийным кодом.
	ConfirmByRecovery struct {
		maxAttempts int16
		expiry      time.Duration
	}
)

// NewConfirmByRecovery - создаёт объект ConfirmByRecovery.
func NewConfirmByRecovery(opts ...Option) *ConfirmByRecovery {
	o := newConfirmOptions(opts)

	return &ConfirmByRecovery{
		maxAttempts: o.maxAttempts,
		expiry:      o.expiry,
	}
}

// Create - создаёт действие подтверждения аварийным кодом. Такое действие ставится только
// завершающим звеном цепочки: аварийный код гасится в момент предъявления, поэтому предъявить
// его раньше, чем комбинация доказательств принята целиком, нельзя.
func (a *ConfirmByRecovery) Create() secureoperation.ConfirmAction {
	return secureoperation.ConfirmAction{
		Method:      confirmmethod.Recovery,
		MaxAttempts: a.maxAttempts,
		Expiry:      a.expiry,
	}
}
