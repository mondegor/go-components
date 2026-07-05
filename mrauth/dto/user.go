package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrtype"

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
	// Теги json синхронизированы с entity.AuthTokenScopes, т.к. структура
	// читается из jsonb-колонки token_scopes; UserID/SessionID хранятся в
	// отдельных полях БД и в jsonb не сериализуются.
	UserScopes struct {
		UserID    uuid.UUID `json:"-"`
		SessionID uint32    `json:"-"`
		Realm     string    `json:"realm"` // domain + '/' + user_group
		Kind      string    `json:"kind"`
		LangCode  string    `json:"lang"`
		// Email    string
		// Phone    uint64
	}

	// UserActivityLastVisited - информация о последнем посещении пользователя.
	UserActivityLastVisited struct {
		UserID        uuid.UUID
		LastVisitedAt time.Time
	}

	// SessionLastActivity - информация о последней активности сессии (для async обновления).
	SessionLastActivity struct {
		UserID        uuid.UUID
		SessionID     uint32
		LastIP        uint32
		LastVisitedAt time.Time
	}

	// UserInfo - сгруппированная информация о пользователе.
	UserInfo struct {
		User    entity.User
		Stat    entity.UserActivityStat
		Auth2FA entity.Auth2FA
		Realms  []entity.UserRealm
	}

	// UserActivityLogMessage - информация об активности пользователя.
	UserActivityLogMessage struct {
		UserID        uuid.UUID         `json:"user_id"`
		SessionID     uint32            `json:"session_id"`
		UserIP        mrtype.DetailedIP `json:"user_ip"`
		UserAgent     string            `json:"user_agent"`
		RequestPath   string            `json:"request_path"`
		RequestStatus uint32            `json:"request_status"`
		VisitedAt     time.Time         `json:"visited_at"`
	}
)
