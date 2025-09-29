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
	// AuthToken - сообщение для получателя.
	AuthToken struct {
		RefreshToken    string
		AccessToken     string
		AccessExpiresAt time.Time
		Scopes          AuthTokenScopes
		ExpiresAt       time.Time
	}

	// AuthTokenScopes - comment struct.
	AuthTokenScopes struct {
		Realm    string    `json:"realm"` // domain + '/' + user_group
		UserKind string    `json:"kind"`
		LangCode string    `json:"lang"`
		UserID   uuid.UUID `json:"-"`
	}
)
