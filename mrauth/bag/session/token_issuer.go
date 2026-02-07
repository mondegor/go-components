package session

import (
	"time"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// TokenIssuer - comment struct.
	TokenIssuer struct {
		tokenGenerator mrauth.TokenGenerator
		accessExpiry   time.Duration
		refreshExpiry  time.Duration
	}
)

// NewTokenIssuer - создаёт объект Session.
func NewTokenIssuer(
	tokenGenerator mrauth.TokenGenerator,
	accessExpiry time.Duration,
	refreshExpiry time.Duration,
) *TokenIssuer {
	return &TokenIssuer{
		tokenGenerator: tokenGenerator,
		accessExpiry:   accessExpiry,
		refreshExpiry:  refreshExpiry,
	}
}

// Create - comments method.
func (uc *TokenIssuer) Create(userScopes dto.UserScopes) (token dto.AuthToken, err error) {
	accessToken, err := uc.tokenGenerator.GenToken()
	if err != nil {
		return dto.AuthToken{}, err
	}

	refreshToken, err := uc.tokenGenerator.GenToken()
	if err != nil {
		return dto.AuthToken{}, err
	}

	return dto.AuthToken{
		AccessToken:      accessToken,
		ExpiresIn:        uc.accessExpiry,
		HasSignature:     false,
		RefreshToken:     refreshToken,
		RefreshExpiresIn: uc.refreshExpiry,
		UserID:           userScopes.UserID,
		Scopes: entity.AuthTokenScopes{
			Realm:    userScopes.Realm,
			UserKind: userScopes.Kind,
			LangCode: userScopes.LangCode,
		},
	}, nil
}
