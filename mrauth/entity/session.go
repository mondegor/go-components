package entity

import (
	"github.com/google/uuid"
)

const (
	// ModelNameSession - название сущности.
	ModelNameSession = "mrauth.Session"
)

type (
	// Session - строка пользовательской сессии.
	Session struct {
		UserID    uuid.UUID
		SessionID uint32
		UserAgent string
		LastIP    uint32 // числовой realIP
	}
)
