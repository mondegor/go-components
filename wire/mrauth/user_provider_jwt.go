package mrauth

import (
	"github.com/mondegor/go-core/mraccess"

	"github.com/mondegor/go-components/mrauth/bag/jwt/crypt"
	"github.com/mondegor/go-components/mrauth/infra/adapter/tokenauth"
	"github.com/mondegor/go-components/mrauth/repository"
)

// NewUserProviderJWT - создаёт провайдер пользователя по access токену в формате JWT.
func NewUserProviderJWT(
	userGroupRights mraccess.RightsGetter,
	jwtKeys crypt.KeySet,
	allowedRealms []string,
) *tokenauth.UserProvider {
	return tokenauth.New(
		repository.NewAuthTokenJWT(jwtKeys),
		userGroupRights,
		allowedRealms,
	)
}
