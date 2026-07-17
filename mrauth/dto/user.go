package dto

import (
	"net/netip"
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

	// UserActivityLastVisited - информация о последнем посещении пользователя в рамках realm'а.
	UserActivityLastVisited struct {
		UserID        uuid.UUID
		RealmID       uint16
		LastVisitedAt time.Time
	}

	// SessionLastActivity - информация о последней активности сессии (для async обновления).
	SessionLastActivity struct {
		UserID        uuid.UUID
		SessionID     uint32
		LastIP        netip.Addr
		LastVisitedAt time.Time
	}

	// UserInfo - сгруппированная информация о пользователе.
	UserInfo struct {
		User    entity.User
		Auth2FA entity.Auth2FA
		Realms  []UserRealmInfo
	}

	// UserRealmInfo - привязка пользователя к realm'у вместе со статистикой последнего входа.
	// LastLocation - местоположение последнего входа (человекочитаемый IP, если не резолвится);
	// LastLoggedAt - время последнего входа в этот realm.
	UserRealmInfo struct {
		RealmID      uint16
		Kind         string
		LastLocation string
		LastLoggedAt time.Time
		CreatedAt    time.Time
		UpdatedAt    time.Time
	}

	// UserActivityLogMessage - информация об активности пользователя.
	// RealmID = 0 - сентинел "realm не определён" (реестр realm'ов разошёлся с провайдерами
	// пользователей, см. produce.UserRequest.Emit): сессия и журнал обрабатываются как обычно,
	// per-realm статистика для такого сообщения не ведётся. В конфиге realm id = 0 запрещён
	// (config.ValidateRealms), поэтому с настоящим realm'ом сентинел не пересекается.
	UserActivityLogMessage struct {
		UserID        uuid.UUID         `json:"user_id"`
		RealmID       uint16            `json:"realm_id"`
		SessionID     uint32            `json:"session_id"`
		UserIP        mrtype.DetailedIP `json:"user_ip"`
		UserAgent     string            `json:"user_agent"`
		RequestPath   string            `json:"request_path"`
		RequestStatus uint32            `json:"request_status"`
		VisitedAt     time.Time         `json:"visited_at"`
	}
)
