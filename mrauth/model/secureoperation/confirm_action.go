package secureoperation

import (
	"time"

	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
)

type (
	// ConfirmAction - способ подтверждения личности пользователя, хранится в виде json.
	ConfirmAction struct {
		Method        confirmmethod.Enum `json:"method"` // email (отправить событие), password, phone (отправить событие), TOTP
		MaxAttempts   int16              `json:"max_attempts"`
		MaxResends    int16              `json:"max_resends,omitempty"`
		MinResendTime time.Duration      `json:"min_resend_time,omitempty"`
		Expiry        time.Duration      `json:"expiry"`

		// AllowRecovery - вместо основного доказательства этого действия допускается предъявить аварийный код.
		AllowRecovery bool `json:"allow_recovery,omitempty"`

		// Address - only for confirmmethod.Email and confirmmethod.Phone.
		Address string `json:"address,omitempty"`

		// ConfirmCode - bcrypt-хеш кода подтверждения; заполняется только у Email/Phone,
		// у остальных действий доказательство сверяет второй фактор, а не операция.
		ConfirmCode string `json:"code,omitempty"`

		// PlainConfirmCode - код подтверждения в открытом виде, используется только для
		// отправки пользователю в рамках текущего запроса; не сохраняется в хранилище.
		PlainConfirmCode string `json:"-"`
	}
)

// Sendable - сообщает, отправляется ли код подтверждения пользователю (Email/Phone).
func (a *ConfirmAction) Sendable() bool {
	return a.Method == confirmmethod.Email || a.Method == confirmmethod.Phone
}
