package pub

import (
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-sysmess/mrstorage/mrsql"

	module "github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/repository"
)

func initUserPostgres(
	dbConnManager mrstorage.DBConnManager,
) *repository.UserPostgres {
	return repository.NewUserPostgres(
		dbConnManager,
		mrsql.DBTableInfo{
			Name:       module.DBSchema + ".users",
			PrimaryKey: "user_id",
		},
	)
}

func initCheckUserPostgres(
	dbConnManager mrstorage.DBConnManager,
) *repository.CheckUserPostgres {
	return repository.NewCheckUserPostgres(
		dbConnManager,
		mrsql.DBTableInfo{
			Name:       module.DBSchema + ".users",
			PrimaryKey: "user_id",
		},
	)
}

func initUserRealmPostgres(
	dbConnManager mrstorage.DBConnManager,
) *repository.UserRealmPostgres {
	return repository.NewUserRealmPostgres(
		dbConnManager,
		module.DBSchema+".users_realms",
	)
}

func initAuth2faPostgres(
	dbConnManager mrstorage.DBConnManager,
) *repository.Auth2faPostgres {
	return repository.NewAuth2faPostgres(
		dbConnManager,
		mrsql.DBTableInfo{
			Name:       module.DBSchema + ".users_auth_2fa",
			PrimaryKey: "user_id",
		},
	)
}

func initUserActivityStatPostgres(
	dbConnManager mrstorage.DBConnManager,
) *repository.UserActivityStatPostgres {
	return repository.NewUserActivityStatPostgres(
		dbConnManager,
		mrsql.DBTableInfo{
			Name:       module.DBSchema + ".users_activity_stat",
			PrimaryKey: "user_id",
		},
	)
}

// func initUserActivityLogPostgres(
//	 dbConnManager mrstorage.DBConnManager,
// ) *repository.UserActivityLogPostgres {
//	 return repository.NewUserActivityLogPostgres(
//		 dbConnManager,
//		 module.DBSchema+".secure_operations_log",
//	 )
// }

func initAuthTokenPostgres(
	dbConnManager mrstorage.DBConnManager,
) *repository.AuthTokenPostgres {
	return repository.NewAuthTokenPostgres(
		dbConnManager,
		mrsql.DBTableInfo{
			Name:       module.DBSchema + ".auth_tokens",
			PrimaryKey: "auth_token",
		},
	)
}

func initSecureOperationPostgres(
	dbConnManager mrstorage.DBConnManager,
) *repository.SecureOperationPostgres {
	return repository.NewSecureOperationPostgres(
		dbConnManager,
		mrsql.DBTableInfo{
			Name:       module.DBSchema + ".secure_operations",
			PrimaryKey: "operation_token",
		},
	)
}

// func initSecureOperationLogPostgres(
//	 dbConnManager mrstorage.DBConnManager,
// ) *repository.SecureOperationLogPostgres {
//	 return repository.NewSecureOperationLogPostgres(
//		 dbConnManager,
//		 module.DBSchema+".secure_operations_log",
//	 )
// }
