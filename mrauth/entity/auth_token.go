package entity

import (
	"time"

	"github.com/google/uuid"
)

const (
	// ModelNameAuthToken - название сущности.
	ModelNameAuthToken = "mrauth.AuthToken" //nolint:gosec

	// ModelNameRefreshToken - название сущности.
	ModelNameRefreshToken = "mrauth.RefreshToken"
)

type (
	// AuthToken - токен доступа пользователя к системе.
	AuthToken struct {
		RefreshToken    string
		AccessToken     string
		AccessExpiresAt time.Time
		UserID          uuid.UUID
		Scopes          AuthTokenScopes
		ExpiresAt       time.Time
	}

	// AuthTokenScopes - область действия токена доступа, хранится в виде json.
	AuthTokenScopes struct {
		Realm    string `json:"realm"` // domain + '/' + user_group
		UserKind string `json:"kind"`
		LangCode string `json:"lang"`
	}
)
