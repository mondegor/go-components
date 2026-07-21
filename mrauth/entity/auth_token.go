package entity

import (
	"time"

	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth/enum/authtokentype"
)

type (
	// AuthToken - токен доступа пользователя к системе (access, refresh или API).
	AuthToken struct {
		Token     string
		Type      authtokentype.Enum
		UserID    uuid.UUID
		RealmID   uint16
		SessionID uint32
		Scopes    AuthTokenScopes
		ExpiresAt time.Time
	}

	// AuthTokenScopes - область действия токена доступа, хранится в виде json.
	AuthTokenScopes struct {
		Realm    string `json:"realm"` // domain + '/' + user_group
		UserKind string `json:"kind"`
		LangCode string `json:"lang"`
		TimeZone string `json:"tz"`
	}
)
