package entity

import (
	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
)

type (
	// Auth2FA - данные второго фактора пользователя (тип, секрет, аварийные коды).
	Auth2FA struct {
		UserID        uuid.UUID
		Type          auth2fatype.Enum
		Secret        string
		LastTOTPStep  int64 // номер последнего использованного TOTP time-step (защита от replay, только TOTP)
		RecoveryCodes []string
	}
)
