package session

import (
	"time"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
)

//go:generate go tool mockgen -destination=mock/mrauth.go -package=mock github.com/mondegor/go-components/mrauth TokenGenerator

type (
	// TokenIssuer - выпускает пару токенов с непрозрачным (сессионным) access токеном.
	TokenIssuer struct {
		tokenGenerator mrauth.TokenGenerator
		accessExpiry   time.Duration
		refreshExpiry  time.Duration
	}
)

// NewTokenIssuer - создаёт объект TokenIssuer.
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

// CreateTokenPair - выпускает пару токенов (сессионный access + refresh) для области действия пользователя.
// TODO: вместо dto.UserScopes можно передавать явно все параметры.
func (uc *TokenIssuer) CreateTokenPair(userScopes dto.UserScopes) (token dto.AuthTokenPair, err error) {
	accessToken, err := uc.createAccessToken()
	if err != nil {
		return dto.AuthTokenPair{}, err
	}

	refreshToken, err := uc.createRefreshToken()
	if err != nil {
		return dto.AuthTokenPair{}, err
	}

	return dto.AuthTokenPair{
		Access:  accessToken,
		Refresh: refreshToken,
		UserID:  userScopes.UserID,
		Scopes: entity.AuthTokenScopes{
			Realm:    userScopes.Realm,
			UserKind: userScopes.Kind,
			LangCode: userScopes.LangCode,
		},
	}, nil
}

func (uc *TokenIssuer) createAccessToken() (dto.AccessToken, error) {
	accessToken, err := uc.tokenGenerator.GenToken()
	if err != nil {
		return dto.AccessToken{}, err
	}

	return dto.AccessToken{
		Token:     accessToken,
		ExpiresIn: uc.accessExpiry,
	}, nil
}

func (uc *TokenIssuer) createRefreshToken() (token dto.RefreshToken, err error) {
	refreshToken, err := uc.tokenGenerator.GenToken()
	if err != nil {
		return dto.RefreshToken{}, err
	}

	return dto.RefreshToken{
		Token:     refreshToken,
		ExpiresIn: uc.refreshExpiry,
	}, nil
}
