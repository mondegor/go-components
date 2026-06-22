package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/mrtype"

	"github.com/mondegor/go-components/mrauth/enum/userstatus"
)

type (
	// User - пользователь.
	User struct {
		ID       uuid.UUID
		Email    string
		Phone    uint64
		LangCode string
		Status   userstatus.Enum
	}

	// UserRealm - привязка пользователя к зоне действия.
	UserRealm struct {
		UserID uuid.UUID
		Realm  string
		Kind   string
	}

	// UserActivityLog - информация об активности пользователя.
	UserActivityLog struct {
		RecordID      uint64
		UserID        uuid.UUID
		UserIP        mrtype.DetailedIP
		UserAgent     string
		RequestPath   string
		RequestStatus uint32
		VisitedAt     time.Time
	}

	// UserActivityStat - информация о последней активности пользователя.
	UserActivityStat struct {
		UserID        uuid.UUID
		LastLoginIP   mrtype.DetailedIP
		LastLoggedAt  time.Time
		LastVisitedAt time.Time
	}
)
