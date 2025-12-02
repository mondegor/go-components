package dto

import (
	"time"

	"github.com/google/uuid"
)

type (
	// User - информация о группе и языке пользователя.
	User struct {
		ID       uuid.UUID
		Group    string
		LangCode string
	}

	// UserInRealm - информация о группе, типе и языке пользователя.
	UserInRealm struct {
		ID       uuid.UUID
		Realm    string
		Kind     string
		LangCode string
		// Email    string
		// Phone    uint64
	}

	// UserActivityLastVisited - информация о последнем посещении пользователя.
	UserActivityLastVisited struct {
		UserID        uuid.UUID
		LastVisitedAt time.Time
	}
)
