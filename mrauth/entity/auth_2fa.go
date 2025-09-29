package entity

import (
	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth/enum"
)

const (
	// ModelNameAuth2fa - название сущности.
	ModelNameAuth2fa = "mrauth.Auth2fa"
)

type (
	// Auth2fa - сообщение для получателя.
	Auth2fa struct {
		UserID       uuid.UUID
		Type         enum.Auth2faType
		Secret       string
		CancelSecret string
	}

	// Auth2faCancel - comment struct.
	Auth2faCancel struct {
		Secret string
	}
)
