package pub

import (
	"github.com/mondegor/go-core/mrevent"
	"github.com/mondegor/go-core/mrlock"
	"github.com/mondegor/go-core/mrlog"
	"github.com/mondegor/go-core/mrstorage"
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
	sessionSoftThreshold, sessionHardThreshold int8,
	appResolver module.AppResolver, // OPTIONAL
	locationResolver module.LocationResolver, // OPTIONAL
	authTokensTableName,
	secureOperationTableName,
	// secureOperationLogTableName,
	sessionsTableName,
	sessionsExcessQueueTableName,
	usersTableName,
	// usersActivityLogTableName,
	usersActivityStatTableName,
	usersAuth2faTableName,
	usersRealmsTableName string,
	debugFunc func(value any) string,
) initing.HttpModule {
	storageAuthToken := initAuthTokenPostgres(dbConnManager, authTokensTableName)
	storageSessionExcessQueue := initSessionExcessQueuePostgres(dbConnManager, sessionsExcessQueueTableName)
	storageSecureOperation := initSecureOperationPostgres(dbConnManager, secureOperationTableName)
	// storageSecureOperationLog := initSecureOperationLogPostgres(dbConnManager, secureOperationLogTableName)
	storageSession := initSessionPostgres(dbConnManager, sessionsTableName)
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
						storageSessionExcessQueue,
						storageSecureOperation,
						useCaseConfirmOperation,
						locker,
						requestParser,
						responseSender,
						notifierAPI,
						userRealms,
						jwtConfig,
						cookieConfig,
						sessionSoftThreshold,
						sessionHardThreshold,
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
						userRealms,
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
						userRealms,
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
						storageUserRealm,
						requestParser,
						responseSender,
						appResolver,
						locationResolver,
						userRealms,
						jwtConfig.Verifier,
					)
				},
			},
		},
	}
}
