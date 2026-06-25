package entity

import (
	"time"

	"github.com/google/uuid"
)

type (
	// Session - строка пользовательской сессии.
	Session struct {
		UserID    uuid.UUID
		SessionID uint32
		UserAgent string
		LastIP    uint32    // числовой realIP
		CreatedAt time.Time // время создания сессии
		UpdatedAt time.Time // время последней активности сессии
	}

	// SessionPK - составной первичный ключ строки сессии (user_id, session_id).
	// Используется как элемент очереди на удаление осиротевших сессий.
	SessionPK struct {
		UserID    uuid.UUID
		SessionID uint32
	}
)
