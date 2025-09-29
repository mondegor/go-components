package session

import (
	"time"

	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
)

type (
	// TokenIssuer - компонент для извлечения настроек, которые хранятся в хранилище данных.
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
func (uc *TokenIssuer) Create(realm, userKind, langCode string, userID uuid.UUID) (token dto.AuthToken, err error) {
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
		Scopes: dto.AuthTokenScopes{
			Realm:    realm,
			UserKind: userKind,
			LangCode: langCode,
			UserID:   userID,
		},
	}, nil
}
