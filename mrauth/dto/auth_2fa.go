package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
)

type (
	// User2FA - данные пользователя для подтверждения второго фактора.
	User2FA struct {
		ID        uuid.UUID
		Email     string
		Phone     uint64
		Action2FA ConfirmAction2FA
	}

	// ConfirmAction2FA - описание активного второго фактора пользователя, из которого
	// строится действие подтверждения защищённой операции.
	ConfirmAction2FA struct {
		Method      confirmmethod.Enum // password, TOTP
		MaxAttempts int16
		Expiry      time.Duration
	}

	// TOTPGeneratorSecret - заготовка TOTP-генератора в текстовом виде: secret для ручного
	// ввода в приложение и otpauth-ссылка с теми же параметрами, что закодированы в QR-коде.
	TOTPGeneratorSecret struct {
		Secret     string
		OTPAuthURI string
	}
)
