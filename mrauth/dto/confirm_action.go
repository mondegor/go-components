package dto

import (
	"time"

	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
)

type (
	// ConfirmAction - comment struct.
	ConfirmAction struct {
		Method        confirmmethod.Enum `json:"method"` // email (отправить событие), password, phone (отправить событие), TOTP
		MaxAttempts   uint32             `json:"maxAttempts"`
		MaxResends    uint32             `json:"maxResends,omitempty"`
		MinResendTime time.Duration      `json:"minResendTime,omitempty"`
		Expiry        time.Duration      `json:"expiry"`
		Address       string             `json:"address,omitempty"`

		// omitempty - ????, hash(пароль) брать у юзера, hash(TOTP) брать у юзера, email одноразовый код, phone одноразовый код
		Secret    string `json:"secret,omitempty"`
		Confirmed bool   `json:"confirmed"`
	}
)
