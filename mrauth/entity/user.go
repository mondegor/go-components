package entity

import (
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrtype"

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

		RegisteredIP mrtype.DetailedIP // IP на момент создания аккаунта, фиксируется однократно (write-once)
	}

	// UserRealm - привязка пользователя к зоне действия.
	UserRealm struct {
		UserID    uuid.UUID
		RealmID   uint16
		Kind      string
		CreatedAt time.Time
		UpdatedAt time.Time
	}

	// UserActivityStat - информация о последней активности пользователя в рамках realm'а.
	UserActivityStat struct {
		UserID        uuid.UUID
		RealmID       uint16
		LastLoginIP   netip.Addr
		LastLoggedAt  time.Time
		LastVisitedAt time.Time
	}
)
