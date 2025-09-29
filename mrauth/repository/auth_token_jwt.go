package repository

import (
	"context"

	"github.com/mondegor/go-components/mrauth/bag/jwt"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// AuthTokenJWT - репозиторий для хранения сообщений подготовленных для отправки различным получателям.
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
func (re *AuthTokenJWT) FetchOne(_ context.Context, accessToken string) (row entity.AuthTokenScopes, err error) {
	scopes, err := re.parser.Parse(accessToken)
	if err != nil {
		return entity.AuthTokenScopes{}, err
	}

	return entity.AuthTokenScopes{
		Realm:    scopes.Realm,
		UserKind: scopes.UserKind,
		LangCode: scopes.LangCode,
		UserID:   scopes.UserID,
	}, nil
}
