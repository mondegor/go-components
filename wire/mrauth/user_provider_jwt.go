package mrauth

import (
	"github.com/mondegor/go-sysmess/mraccess"

	"github.com/mondegor/go-components/mrauth/component/get"
	"github.com/mondegor/go-components/mrauth/repository"
)

// NewUserProviderJWT - создаёт получателя произвольных настроек из БД.
func NewUserProviderJWT(
	userGroups mraccess.RightsGetter,
	secret string,
	allowedRealms []string,
) *get.UserProvider {
	return get.New(
		repository.NewAuthTokenJWT(secret),
		userGroups,
		allowedRealms,
	)
}
