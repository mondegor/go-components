package entity

import (
	"net/netip"
	"time"

	"github.com/google/uuid"
)

type (
	// Session - строка пользовательской сессии.
	Session struct {
		UserID    uuid.UUID
		SessionID uint32
		UserAgent string
		LastIP    netip.Addr // realIP последней активности
		CreatedAt time.Time  // время создания сессии
		UpdatedAt time.Time  // время последней активности сессии
	}

	// SessionPK - составной первичный ключ строки сессии (user_id, session_id).
	// Используется как элемент очереди на удаление осиротевших сессий.
	SessionPK struct {
		UserID    uuid.UUID
		SessionID uint32
	}

	// SessionExcessItem - элемент очереди на фоновую чистку лишних сессий пользователя в realm.
	SessionExcessItem struct {
		UserID     uuid.UUID
		RealmID    uint16
		SessionMax int // лимит одновременных сессий realm, зафиксированный на момент постановки в очередь
	}

	// SessionExcessPK - составной ключ строки очереди чистки лишних сессий (user_id, realm_id).
	// Используется как элемент ack обработанной пачки.
	SessionExcessPK struct {
		UserID  uuid.UUID
		RealmID uint16
	}
)
