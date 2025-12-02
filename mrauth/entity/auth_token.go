package entity

import (
	"time"

	"github.com/mondegor/go-components/mrauth/dto"
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
		Scopes          dto.AuthTokenScopes
		ExpiresAt       time.Time
	}
)
