package dto

import (
	"time"

	"github.com/google/uuid"
)

type (
	// AuthToken - сообщение для получателя.
	AuthToken struct {
		AccessToken      string
		ExpiresIn        time.Duration
		HasSignature     bool
		RefreshToken     string
		RefreshExpiresIn time.Duration
		Scopes           AuthTokenScopes
	}

	// AuthTokenScopes - comment struct.
	AuthTokenScopes struct {
		Realm    string
		UserKind string
		LangCode string
		UserID   uuid.UUID
	}
)
