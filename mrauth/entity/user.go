package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/mrtype"

	"github.com/mondegor/go-components/mrauth/enum/userstatus"
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

	// UserActivityStat - информация о последней активности пользователя.
	UserActivityStat struct {
		UserID        uuid.UUID
		LastLoginIP   mrtype.DetailedIP
		LastLoggedAt  time.Time
		LastVisitedAt time.Time
	}

	// UserActivityLog - информация об активности пользователя.
	UserActivityLog struct {
		RecordID      uint64            `json:"record_id"`
		UserID        uuid.UUID         `json:"user_id"`
		UserIP        mrtype.DetailedIP `json:"user_ip"`
		UserAgent     string            `json:"user_agent"`
		RequestPath   string            `json:"request_path"`
		RequestStatus uint32            `json:"request_status"`
		VisitedAt     time.Time         `json:"visited_at"`
	}

	// UserInfo - сгруппированная информация о пользователе.
	UserInfo struct {
		User    User
		Stat    UserActivityStat
		Auth2fa Auth2fa
		Realms  []UserRealm
	}
)
