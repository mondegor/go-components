package repository

import (
	"context"

	"github.com/mondegor/go-components/mrauth/bag/jwt"
	"github.com/mondegor/go-components/mrauth/dto"
)

type (
	// AuthTokenJWT - распаковка области действия пользователя из подписанного JWT access токена.
	AuthTokenJWT struct {
		parser *jwt.Parser
	}
)

// NewAuthTokenJWT - создаёт объект AuthTokenJWT.
func NewAuthTokenJWT(secret string) *AuthTokenJWT {
	return &AuthTokenJWT{
		parser: jwt.NewParser(secret),
	}
}

// FetchOneByAccessToken - возвращает область действия пользователя по access токену.
func (re *AuthTokenJWT) FetchOneByAccessToken(_ context.Context, accessToken string) (row dto.UserScopes, err error) {
	scopes, err := re.parser.Parse(accessToken)
	if err != nil {
		return dto.UserScopes{}, err
	}

	return scopes, nil
}
