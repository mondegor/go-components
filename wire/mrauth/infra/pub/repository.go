package pub

import (
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrauth/repository"
)

func initUserPostgres(
	dbConnManager mrstorage.DBConnManager,
	tableName string,
) *repository.UserPostgres {
	return repository.NewUserPostgres(dbConnManager, tableName)
}

func initCheckUserPostgres(
	dbConnManager mrstorage.DBConnManager,
	tableName string,
) *repository.CheckUserPostgres {
	return repository.NewCheckUserPostgres(dbConnManager, tableName)
}

func initUserRealmPostgres(
	dbConnManager mrstorage.DBConnManager,
	tableName string,
) *repository.UserRealmPostgres {
	return repository.NewUserRealmPostgres(dbConnManager, tableName)
}

func initAuth2faPostgres(
	dbConnManager mrstorage.DBConnManager,
	tableName string,
) *repository.Auth2FAPostgres {
	return repository.NewAuth2FAPostgres(dbConnManager, tableName)
}

func initUserActivityStatPostgres(
	dbConnManager mrstorage.DBConnManager,
	tableName string,
) *repository.UserActivityStatPostgres {
	return repository.NewUserActivityStatPostgres(dbConnManager, tableName)
}

// func initUserActivityLogPostgres(
// 	dbConnManager mrstorage.DBConnManager,
// 	tableName string,
// ) *repository.UserActivityLogPostgres {
// 	return repository.NewUserActivityLogPostgres(dbConnManager, tableName)
// }

func initSessionPostgres(
	dbConnManager mrstorage.DBConnManager,
	tableName string,
) *repository.SessionPostgres {
	return repository.NewSessionPostgres(dbConnManager, tableName)
}

func initAuthTokenPostgres(
	dbConnManager mrstorage.DBConnManager,
	tableName string,
) *repository.AuthTokenPostgres {
	return repository.NewAuthTokenPostgres(dbConnManager, tableName)
}

func initSessionExcessQueuePostgres(
	dbConnManager mrstorage.DBConnManager,
	tableName string,
) *repository.SessionExcessQueuePostgres {
	return repository.NewSessionExcessQueuePostgres(dbConnManager, tableName)
}

func initSecureOperationPostgres(
	dbConnManager mrstorage.DBConnManager,
	tableName string,
) *repository.SecureOperationPostgres {
	return repository.NewSecureOperationPostgres(dbConnManager, tableName)
}
