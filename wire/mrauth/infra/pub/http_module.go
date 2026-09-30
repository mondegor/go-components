package pub

import (
	"context"
	"time"

	"github.com/mondegor/go-core/mrevent"
	"github.com/mondegor/go-core/mrlock"
	"github.com/mondegor/go-core/mrlog"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-webcore/mrcore/initing"
	"github.com/mondegor/go-webcore/mrserver"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/infra/adapter/collect"
	"github.com/mondegor/go-components/mrauth/service/notify"
	"github.com/mondegor/go-components/mrauth/service/secureoperation"
	"github.com/mondegor/go-components/mrauth/validate"
	authcfg "github.com/mondegor/go-components/wire/mrauth/config"
)

type (
	// SecureOperationLogProducer - продюсер записей журнала защищённых операций,
	// поставляемый хостом (обычно коллектор wire/mrauth/oplogger/collector).
	SecureOperationLogProducer interface {
		PushMessage(ctx context.Context, entry entity.SecureOperationLog) error
	}
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
	notifierAPI mrauth.Notifier,
	secureOperationLogProducer SecureOperationLogProducer,
	userRealms []authcfg.UserRealm,
	operationConfig authcfg.OperationConfirm,
	auth2faConfig authcfg.Auth2FA,
	jwtConfig authcfg.JWT,
	cookieConfig authcfg.RefreshCookie,
	sessionSoftThreshold, sessionHardThreshold int8,
	sessionLimitRetryAfter time.Duration,
	appResolver mrauth.AppResolver, // OPTIONAL
	locationResolver mrauth.LocationResolver, // OPTIONAL
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

	operationLogger := collect.NewSecureOperationLogger(secureOperationLogProducer, logger)

	// контекст клиента (время, IP, устройство) в уведомлениях о событиях безопасности
	actorProps := notify.NewActorProps(appResolver)

	// единая точка открытия защищённых операций всех типов (гасит прежние операции того же типа или той же цепочки)
	operationOpener := secureoperation.NewOpener(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		operationLogger,
	)

	useCaseConfirmOperation := initConfirmOperationUseCase(
		dbConnManager,
		storageSecureOperation,
		storageAuth2fa,
		storageUser,
		notifierAPI,
		actorProps,
		operationLogger,
		auth2faConfig,
	)

	return initing.HttpModule{
		Caption:    mrauth.Name,
		Permission: mrauth.Permission,
		Controllers: []initing.HttpController{
			{
				Create: func() (mrserver.HttpController, error) {
					return initUnitAuthController(
						logger,
						eventEmitter,
						dbConnManager,
						operationOpener,
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
						operationLogger,
						locker,
						requestParser,
						responseSender,
						notifierAPI,
						actorProps,
						userRealms,
						auth2faConfig,
						jwtConfig,
						cookieConfig,
						sessionSoftThreshold,
						sessionHardThreshold,
						sessionLimitRetryAfter,
						debugFunc,
						locationResolver,
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
						auth2faConfig,
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
						operationLogger,
						requestParser,
						responseSender,
						notifierAPI,
						debugFunc,
					)
				},
			},
			{
				Create: func() (mrserver.HttpController, error) {
					return initSecurityController(
						dbConnManager,
						operationOpener,
						storageUser,
						storageCheckUser,
						storageUserRealm,
						storageAuth2fa,
						storageSecureOperation,
						operationLogger,
						requestParser,
						responseFileSender,
						notifierAPI,
						actorProps,
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
