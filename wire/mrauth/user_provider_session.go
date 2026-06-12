package mrauth

import (
	"github.com/mondegor/go-sysmess/mraccess"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-sysmess/mrstorage/mrsql"

	"github.com/mondegor/go-components/mrauth/component/get"
	"github.com/mondegor/go-components/mrauth/repository"
)

// NewUserProviderSession - создаёт получателя произвольных настроек из БД.
func NewUserProviderSession(
	client mrstorage.DBConnManager,
	userGroupRights mraccess.RightsGetter,
	storageTable mrsql.DBTableInfo,
	allowedRealms []string,
) *get.UserProvider {
	return get.New(
		repository.NewAuthTokenPostgres(
			client,
			storageTable,
		),
		userGroupRights,
		allowedRealms,
	)
}
