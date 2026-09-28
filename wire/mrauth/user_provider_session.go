package mrauth

import (
	"github.com/mondegor/go-core/mraccess"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrauth/infra/adapter/tokenauth"
	"github.com/mondegor/go-components/mrauth/repository"
)

// NewUserProviderSession - создаёт провайдер пользователя по сессионному access токену, хранимому в БД.
func NewUserProviderSession(
	client mrstorage.DBConnManager,
	userGroupRights mraccess.RightsGetter,
	tableName string,
	allowedRealms []string,
) *tokenauth.UserProvider {
	return tokenauth.New(
		repository.NewAuthTokenPostgres(
			client,
			tableName,
		),
		userGroupRights,
		allowedRealms,
	)
}
