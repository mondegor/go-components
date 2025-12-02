package dto

import (
	"time"

	"github.com/google/uuid"
)

type (
	// AuthToken - токен доступа пользователя к системе.
	AuthToken struct {
		AccessToken      string
		ExpiresIn        time.Duration
		HasSignature     bool
		RefreshToken     string
		RefreshExpiresIn time.Duration
		Scopes           AuthTokenScopes
	}

	// AuthTokenScopes - область действия токена доступа.
	AuthTokenScopes struct {
		Realm    string    `json:"realm"` // domain + '/' + user_group
		UserKind string    `json:"kind"`
		LangCode string    `json:"lang"`
		UserID   uuid.UUID `json:"-"`
	}
)
