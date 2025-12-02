package entity

import (
	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
)

const (
	// ModelNameAuth2fa - название сущности.
	ModelNameAuth2fa = "mrauth.Auth2fa"
)

type (
	// Auth2fa - информация о пользователе для 2FA.
	Auth2fa struct {
		UserID       uuid.UUID
		Type         auth2fatype.Enum
		Secret       string
		CancelSecret string
	}
)
