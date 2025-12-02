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

// FetchOne - возвращает список сообщений по их указанным SettingID.
func (re *AuthTokenJWT) FetchOne(_ context.Context, accessToken string) (row dto.AuthTokenScopes, err error) {
	scopes, err := re.parser.Parse(accessToken)
	if err != nil {
		return dto.AuthTokenScopes{}, err
	}

	return dto.AuthTokenScopes{
		Realm:    scopes.Realm,
		UserKind: scopes.UserKind,
		LangCode: scopes.LangCode,
		UserID:   scopes.UserID,
	}, nil
}
