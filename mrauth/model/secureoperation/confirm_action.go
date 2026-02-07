package secureoperation

import (
	"time"

	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
)

type (
	// ConfirmAction - способ подтверждения личности пользователя, хранится в виде json.
	ConfirmAction struct {
		Method        confirmmethod.Enum `json:"method"` // email (отправить событие), password, phone (отправить событие), TOTP
		MaxAttempts   uint32             `json:"max_attempts"`
		MaxResends    uint32             `json:"max_resends,omitempty"`
		MinResendTime time.Duration      `json:"min_resend_time,omitempty"`
		Expiry        time.Duration      `json:"expiry"`
		Address       string             `json:"address,omitempty"`

		// omitempty - ????, hash(пароль) брать у юзера, hash(TOTP) брать у юзера, email одноразовый код, phone одноразовый код
		Secret    string `json:"secret,omitempty"`
		Confirmed bool   `json:"confirmed"`
	}
)
