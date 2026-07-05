package mrauth

import (
	"github.com/mondegor/go-core/mraccess"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrauth/component/get"
	"github.com/mondegor/go-components/mrauth/repository"
)

// NewUserProviderSession - создаёт получателя произвольных настроек из БД.
func NewUserProviderSession(
	client mrstorage.DBConnManager,
	userGroupRights mraccess.RightsGetter,
	tableName string,
	allowedRealms []string,
) *get.UserProvider {
	return get.New(
		repository.NewAuthTokenPostgres(
			client,
			tableName,
		),
		userGroupRights,
		allowedRealms,
	)
}
