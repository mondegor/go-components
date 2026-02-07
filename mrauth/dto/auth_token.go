package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// AuthToken - токен доступа пользователя к системе.
	AuthToken struct {
		AccessToken      string
		ExpiresIn        time.Duration
		HasSignature     bool
		RefreshToken     string
		RefreshExpiresIn time.Duration
		UserID           uuid.UUID
		Scopes           entity.AuthTokenScopes
	}
)
