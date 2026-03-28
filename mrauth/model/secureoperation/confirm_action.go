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

		// only for confirmmethod.Email and confirmmethod.Phone
		Address     string `json:"address,omitempty"`
		ConfirmCode string `json:"code,omitempty"`
	}
)

// Sendable - comments method.
func (a *ConfirmAction) Sendable() bool {
	return a.Method == confirmmethod.Email || a.Method == confirmmethod.Phone
}
