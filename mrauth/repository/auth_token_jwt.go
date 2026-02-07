package repository

import (
	"context"

	"github.com/mondegor/go-components/mrauth/bag/jwt"
	"github.com/mondegor/go-components/mrauth/dto"
)

type (
	// AuthTokenJWT - comment struct.
	AuthTokenJWT struct {
		parser *jwt.Parser
	}
)

// NewAuthTokenJWT - создаёт объект AuthTokenPostgres.
func NewAuthTokenJWT(secret string) *AuthTokenJWT {
	return &AuthTokenJWT{
		parser: jwt.NewParser(secret),
	}
}

// FetchOne - возвращает список сообщений по их указанным ID.
func (re *AuthTokenJWT) FetchOne(_ context.Context, accessToken string) (row dto.UserScopes, err error) {
	scopes, err := re.parser.Parse(accessToken)
	if err != nil {
		return dto.UserScopes{}, err
	}

	return scopes, nil
}
