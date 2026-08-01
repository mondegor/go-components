package dto

import (
	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// User2FA - данные пользователя для подтверждения второго фактора.
	User2FA struct {
		ID        uuid.UUID
		Email     string
		Phone     uint64
		Action2FA secureoperation.ConfirmAction
	}

	// TOTPGeneratorSecret - заготовка TOTP-генератора в текстовом виде: secret для ручного
	// ввода в приложение и otpauth-ссылка с теми же параметрами, что закодированы в QR-коде.
	TOTPGeneratorSecret struct {
		Secret     string
		OTPAuthURI string
	}
)
