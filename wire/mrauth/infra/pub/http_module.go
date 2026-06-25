package pub

import (
	"github.com/mondegor/go-sysmess/mrevent"
	"github.com/mondegor/go-sysmess/mrlock"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-webcore/mrcore/initing"
	"github.com/mondegor/go-webcore/mrserver"

	module "github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/validate"
	"github.com/mondegor/go-components/mrnotifier"
	authcfg "github.com/mondegor/go-components/wire/mrauth/config"
)

// InitHttpModule - создаёт все компоненты модуля и возвращает его HTTP-контроллеры.
func InitHttpModule(
	logger mrlog.Logger,
	eventEmitter mrevent.Emitter,
	dbConnManager mrstorage.DBConnManager,
	locker mrlock.Locker,
	requestParser *validate.Parser,
	responseSender mrserver.ResponseSender,
	responseFileSender mrserver.FileResponseSender,
	notifierAPI mrnotifier.NoteProducer,
	userRealms []authcfg.UserRealm,
	operationConfig authcfg.OperationConfirm,
	auth2faConfig authcfg.Auth2FA,
	jwtConfig authcfg.JWT,
	cookieConfig authcfg.RefreshCookie,
	appResolver module.AppResolver, // OPTIONAL
	locationResolver module.LocationResolver, // OPTIONAL
	authTokensTableName,
	secureOperationTableName,
	// secureOperationLogTableName,
	sessionsTableName,
	usersTableName,
	// usersActivityLogTableName,
	usersActivityStatTableName,
	usersAuth2faTableName,
	usersRealmsTableName string,
	maxUserSessions uint16,
	debugFunc func(value any) string,
) initing.HttpModule {
	storageAuthToken := initAuthTokenPostgres(dbConnManager, authTokensTableName)
	storageSecureOperation := initSecureOperationPostgres(dbConnManager, secureOperationTableName)
	// storageSecureOperationLog := initSecureOperationLogPostgres(dbConnManager, secureOperationLogTableName)
	storageSession := initSessionPostgres(dbConnManager, sessionsTableName, int(maxUserSessions))
	storageUser := initUserPostgres(dbConnManager, usersTableName)
	storageCheckUser := initCheckUserPostgres(dbConnManager, usersTableName)
	storageUserActivityStat := initUserActivityStatPostgres(dbConnManager, usersActivityStatTableName)
	// storageUserActivityLog := initUserActivityLogPostgres(dbConnManager, usersActivityLogTableName)
	storageAuth2fa := initAuth2faPostgres(dbConnManager, usersAuth2faTableName)
	storageUserRealm := initUserRealmPostgres(dbConnManager, usersRealmsTableName)

	auth2faConfig = authcfg.CorrectValuesAuth2FA(auth2faConfig)

	useCaseConfirmOperation := initConfirmOperationUseCase(
		dbConnManager,
		storageSecureOperation,
		storageAuth2fa,
		notifierAPI,
		operationConfig,
		auth2faConfig,
	)

	return initing.HttpModule{
		Caption:    module.Name,
		Permission: module.Permission,
		Controllers: []initing.HttpController{
			{
				Create: func() (mrserver.HttpController, error) {
					return initUnitAuthController(
						logger,
						eventEmitter,
						dbConnManager,
						storageUser,
						storageCheckUser,
						storageUserRealm,
						storageAuth2fa,
						storageUserActivityStat,
						storageSession,
						storageAuthToken,
						storageSecureOperation,
						useCaseConfirmOperation,
						locker,
						requestParser,
						responseSender,
						notifierAPI,
						userRealms,
						jwtConfig,
						cookieConfig,
						debugFunc,
					)
				},
			},
			{
				Create: func() (mrserver.HttpController, error) {
					return initCheckController(
						storageCheckUser,
						storageUserRealm,
						requestParser,
						responseSender,
						jwtConfig.Verifier,
					)
				},
			},
			{
				Create: func() (mrserver.HttpController, error) {
					return initOperationController(
						dbConnManager,
						storageSecureOperation,
						useCaseConfirmOperation,
						requestParser,
						responseSender,
						notifierAPI,
						operationConfig,
						debugFunc,
					)
				},
			},
			{
				Create: func() (mrserver.HttpController, error) {
					return initSecurityController(
						dbConnManager,
						storageUser,
						storageCheckUser,
						storageUserRealm,
						storageAuth2fa,
						storageSecureOperation,
						requestParser,
						responseFileSender,
						notifierAPI,
						operationConfig,
						auth2faConfig,
						debugFunc,
					)
				},
			},
			{
				Create: func() (mrserver.HttpController, error) {
					return initSessionsController(
						storageSession,
						storageAuthToken,
						requestParser,
						responseSender,
						appResolver,
						locationResolver,
						jwtConfig.Verifier,
					)
				},
			},
		},
	}
}
