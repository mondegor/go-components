package mrauth

import (
	"github.com/mondegor/go-sysmess/mraccess"

	"github.com/mondegor/go-components/mrauth/bag/jwt/crypt"
	"github.com/mondegor/go-components/mrauth/component/get"
	"github.com/mondegor/go-components/mrauth/repository"
)

// NewUserProviderJWT - создаёт получателя произвольных настроек из БД.
func NewUserProviderJWT(
	userGroupRights mraccess.RightsGetter,
	jwtKeys crypt.KeySet,
	allowedRealms []string,
) *get.UserProvider {
	return get.New(
		repository.NewAuthTokenJWT(jwtKeys),
		userGroupRights,
		allowedRealms,
	)
}
