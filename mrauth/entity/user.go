package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/mrtype"

	"github.com/mondegor/go-components/mrauth/enum"
)

const (
	// ModelNameUser - название сущности.
	ModelNameUser = "mrauth.User"

	// ModelNameUserRealm - название сущности.
	ModelNameUserRealm = "mrauth.UserRealm"

	// ModelNameUserLog - название сущности.
	ModelNameUserLog = "mrauth.UserLog"
)

type (
	// User - сообщение для получателя.
	User struct {
		ID       uuid.UUID
		Email    string
		Phone    uint64
		LangCode string
		Status   enum.UserStatus
	}

	// UserRealm - сообщение для получателя.
	UserRealm struct {
		UserID uuid.UUID
		Realm  string
		Kind   string
	}

	// UserActivityStat - сообщение для получателя.
	UserActivityStat struct {
		UserID        uuid.UUID
		LastLoginIP   mrtype.DetailedIP
		LastLoggedAt  time.Time
		LastVisitedAt time.Time
	}

	// UserActivityLastVisited - сообщение для получателя.
	UserActivityLastVisited struct {
		UserID        uuid.UUID
		LastVisitedAt time.Time
	}

	// UserActivityLog - сообщение для получателя.
	UserActivityLog struct {
		RecordID      uint64            `json:"record_id"`
		UserID        uuid.UUID         `json:"user_id"`
		UserIP        mrtype.DetailedIP `json:"user_ip"`
		UserAgent     string            `json:"user_agent"`
		RequestPath   string            `json:"request_path"`
		RequestStatus uint32            `json:"request_status"`
		VisitedAt     time.Time         `json:"visited_at"`
	}
)
