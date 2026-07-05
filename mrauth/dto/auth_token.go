package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// AccessToken - токен доступа пользователя к системе.
	AccessToken struct {
		Token        string
		ExpiresIn    time.Duration
		HasSignature bool
	}

	// RefreshToken - токен обновления пары токенов доступа.
	RefreshToken struct {
		Token     string
		ExpiresIn time.Duration
	}

	// AuthTokenPair - пара токенов access/refresh для доступа пользователя к системе.
	AuthTokenPair struct {
		Access  AccessToken
		Refresh RefreshToken
		UserID  uuid.UUID
		Scopes  entity.AuthTokenScopes
	}
)
