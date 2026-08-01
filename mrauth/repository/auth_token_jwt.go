package repository

import (
	"context"
	"errors"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/jwt"
	"github.com/mondegor/go-components/mrauth/bag/jwt/crypt"
	"github.com/mondegor/go-components/mrauth/dto"
)

type (
	// AuthTokenJWT - распаковка области действия пользователя из подписанного JWT access токена.
	AuthTokenJWT struct {
		parser *jwt.Parser
	}
)

// NewAuthTokenJWT - создаёт объект AuthTokenJWT.
func NewAuthTokenJWT(keys crypt.KeySet) *AuthTokenJWT {
	return &AuthTokenJWT{
		parser: jwt.NewParser(keys),
	}
}

// FetchOneByAccessToken - возвращает область действия пользователя по access токену.
// Если срок действия токена истёк, возвращает mrauth.ErrEventTokenExpired,
// если токен не разобрался - mrauth.ErrTokenInvalid.
func (re *AuthTokenJWT) FetchOneByAccessToken(_ context.Context, accessToken string) (row dto.UserScopes, err error) {
	scopes, err := re.parser.Parse(accessToken)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return dto.UserScopes{}, mrauth.ErrEventTokenExpired
		}

		return dto.UserScopes{}, mrauth.ErrTokenInvalid.Wrap(err)
	}

	return scopes, nil
}
