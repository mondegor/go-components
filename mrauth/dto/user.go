package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/mrtype"

	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// User - информация о группе и языке пользователя.
	User struct {
		ID       uuid.UUID
		Group    string
		LangCode string
	}

	// UserScopes - область действия пользователя.
	UserScopes struct {
		UserID    uuid.UUID
		SessionID uint32
		Realm     string // domain + '/' + user_group
		Kind      string
		LangCode  string
		// Email    string
		// Phone    uint64
	}

	// UserActivityLastVisited - информация о последнем посещении пользователя.
	UserActivityLastVisited struct {
		UserID        uuid.UUID
		LastVisitedAt time.Time
	}

	// UserInfo - сгруппированная информация о пользователе.
	UserInfo struct {
		User    entity.User
		Stat    entity.UserActivityStat
		Auth2fa entity.Auth2fa
		Realms  []entity.UserRealm
	}

	// UserActivityLogMessage - информация об активности пользователя.
	UserActivityLogMessage struct {
		UserID        uuid.UUID         `json:"user_id"`
		UserIP        mrtype.DetailedIP `json:"user_ip"`
		UserAgent     string            `json:"user_agent"`
		RequestPath   string            `json:"request_path"`
		RequestStatus uint32            `json:"request_status"`
		VisitedAt     time.Time         `json:"visited_at"`
	}
)
