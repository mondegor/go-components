package mrauth

import (
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrerr/errorwrapper"

	"github.com/mondegor/go-components/mrauth/component/get"
	"github.com/mondegor/go-components/mrauth/repository"
)

// NewUserProviderSession - создаёт получателя произвольных настроек из БД.
func NewUserProviderSession(
	client mrstorage.DBConnManager,
	storageTable mrsql.DBTableInfo,
	allowedRealms []string,
) *get.UserProvider {
	return get.New(
		repository.NewAuthTokenPostgres(
			client,
			errorwrapper.NewInfraStorage(),
			storageTable,
		),
		errorwrapper.NewUseCase(),
		allowedRealms,
	)
}
