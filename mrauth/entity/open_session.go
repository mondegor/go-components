package entity

import (
	"time"
)

type (
	// OpenSession - открытая сессия пользователя: идентификатор
	// и срок действия её refresh токена (= срок жизни сессии).
	OpenSession struct {
		SessionID uint32
		ExpiresAt time.Time
	}

	// OpenSessions - список открытых сессий пользователя.
	OpenSessions []OpenSession
)

// IDs - возвращает идентификаторы открытых сессий в исходном порядке.
func (s OpenSessions) IDs() []uint32 {
	ids := make([]uint32, 0, len(s))

	for i := range s {
		ids = append(ids, s[i].SessionID)
	}

	return ids
}

// ExpiresAt - возвращает срок действия сессии с указанным id,
// или нулевое время, если такой сессии нет в списке.
func (s OpenSessions) ExpiresAt(sessionID uint32) time.Time {
	for i := range s {
		if s[i].SessionID == sessionID {
			return s[i].ExpiresAt
		}
	}

	return time.Time{}
}
