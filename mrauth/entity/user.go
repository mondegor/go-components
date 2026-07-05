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
		ID        uuid.UUID
		Email     string
		Phone     uint64
		LangCode  string
		Status    userstatus.Enum
		CreatedAt time.Time
		UpdatedAt time.Time
	}

	// ExtendedUser - пользователь с расширенными полями.
	ExtendedUser struct {
		User

		RegisteredIP string // IP на момент создания аккаунта, фиксируется однократно (write-once)
	}

	// UserRealm - привязка пользователя к зоне действия.
	UserRealm struct {
		UserID    uuid.UUID
		RealmID   uint16
		Kind      string
		CreatedAt time.Time
		UpdatedAt time.Time
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
