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
	auth "github.com/mondegor/go-components/wire/mrauth/config"
)

// InitHttpModule - создаются все компоненты модуля и возвращаются к нему контролеры.
func InitHttpModule(
	logger mrlog.Logger,
	eventEmitter mrevent.Emitter,
	dbConnManager mrstorage.DBConnManager,
	locker mrlock.Locker,
	requestParser *validate.Parser,
	responseSender mrserver.ResponseSender,
	responseFileSender mrserver.FileResponseSender,
	notifierAPI mrnotifier.NoteProducer,
	userRealms []auth.UserRealm,
	operationConfirm auth.OperationConfirm,
	jwtConfig auth.JWT,
	debugFunc func(value any) string,
) initing.HttpModule {
	storageUser := initUserPostgres(dbConnManager)
	storageCheckUser := initCheckUserPostgres(dbConnManager)
	storageUserRealm := initUserRealmPostgres(dbConnManager)
	storageAuth2fa := initAuth2faPostgres(dbConnManager)
	storageUserActivityStat := initUserActivityStatPostgres(dbConnManager)
	// storageUserActivityLog := initUserActivityLogPostgres(dbConnManager)
	storageAuthToken := initAuthTokenPostgres(dbConnManager)
	storageSecureOperation := initSecureOperationPostgres(dbConnManager)
	// storageSecureOperationLog := initSecureOperationLogPostgres(dbConnManager)
	useCaseConfirmOperation := initConfirmOperationUseCase(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		operationConfirm,
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
						storageAuthToken,
						storageSecureOperation,
						useCaseConfirmOperation,
						locker,
						requestParser,
						responseSender,
						notifierAPI,
						userRealms,
						jwtConfig,
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
						operationConfirm,
						debugFunc,
					)
				},
			},
			{
				Create: func() (mrserver.HttpController, error) {
					return initSecurityController(
						logger,
						dbConnManager,
						storageUser,
						storageCheckUser,
						storageUserRealm,
						storageAuth2fa,
						storageSecureOperation,
						requestParser,
						responseFileSender,
						notifierAPI,
						debugFunc,
					)
				},
			},
		},
	}
}
