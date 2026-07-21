package pub

import (
	"time"

	"github.com/mondegor/go-core/mrevent"
	"github.com/mondegor/go-core/mrlock"
	"github.com/mondegor/go-core/mrlog"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-webcore/mrserver"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/component/produce"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/bag"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/service"
	"github.com/mondegor/go-components/mrauth/service/authtoken"
	"github.com/mondegor/go-components/mrauth/service/authuser"
	"github.com/mondegor/go-components/mrauth/service/check"
	servicelang "github.com/mondegor/go-components/mrauth/service/lang"
	sessionsrv "github.com/mondegor/go-components/mrauth/service/session"
	servicetimezone "github.com/mondegor/go-components/mrauth/service/timezone"
	"github.com/mondegor/go-components/mrauth/service/userinfo"
	usecaseauth "github.com/mondegor/go-components/mrauth/usecase/auth"
	"github.com/mondegor/go-components/mrauth/usecase/operation"
	"github.com/mondegor/go-components/mrauth/usecase/session"
	"github.com/mondegor/go-components/mrauth/usecase/session/handler"
	usecaseuser "github.com/mondegor/go-components/mrauth/usecase/user"
	"github.com/mondegor/go-components/mrauth/validate"
	"github.com/mondegor/go-components/mrnotifier"
	authcfg "github.com/mondegor/go-components/wire/mrauth/config"
	"github.com/mondegor/go-components/wire/mrauth/mapping"
)

func initUnitAuthController(
	logger mrlog.Logger,
	eventEmitter mrevent.Emitter,
	dbConnManager mrstorage.DBConnManager,
	storageUser *repository.UserPostgres,
	storageCheckUser *repository.CheckUserPostgres,
	storageUserRealm *repository.UserRealmPostgres,
	storageAuth2fa *repository.Auth2FAPostgres,
	storageUserActivityStat *repository.UserActivityStatPostgres,
	storageSession *repository.SessionPostgres,
	storageAuthToken *repository.AuthTokenPostgres,
	storageSessionExcessQueue *repository.SessionExcessQueuePostgres,
	storageSecureOperation *repository.SecureOperationPostgres,
	useCaseConfirmOperation *operation.ConfirmOperation,
	operationLogger *produce.SecureOperationLogger,
	locker mrlock.Locker,
	requestParser *validate.Parser,
	responseSender mrserver.ResponseSender,
	notifierAPI mrnotifier.NoteProducer,
	userRealms []authcfg.UserRealm,
	jwtConfig authcfg.JWT,
	cookieConfig authcfg.RefreshCookie,
	sessionSoftThreshold, sessionHardThreshold int8,
	debugFunc func(value any) string,
	locationResolver mrauth.LocationResolver,
	languages mrauth.LanguageList,
	timeZones mrauth.TimeZoneList,
) (mrserver.HttpController, error) {
	realmRegistry := mapping.OptionUserRealmsToRealmRegistry(userRealms)

	checkUserService := check.NewUserLogin(
		storageCheckUser,
		storageUserRealm,
		realmRegistry,
	)

	factory2FA := service.NewFactoryConfirm2FA(
		storageUser,
		storageAuth2fa,
		action.NewConfirmBy2fa(
			[]action.Option{
				action.WithMaxAttempts(5), // TODO: в настройки
				action.WithExpiry(30 * time.Minute),
			},
			[]action.Option{
				action.WithMaxAttempts(5), // TODO: в настройки
				action.WithExpiry(30 * time.Minute),
			},
		),
	)

	langResolver := servicelang.New(languages)
	timeZoneResolver := servicetimezone.New(timeZones)

	useCaseCreateUser := usecaseauth.NewCreateUser(
		dbConnManager,
		checkUserService,
		storageSecureOperation,
		notifierAPI,
		factory2FA,
		locker,
		operationLogger,
		timeZoneResolver,
		mapping.OptionUserRealmsToConfirmCreateUserRealms(userRealms),
	)

	useCaseConfirmAuthUser := usecaseauth.NewCreateSession(
		dbConnManager,
		checkUserService,
		storageSecureOperation,
		notifierAPI,
		factory2FA,
		operationLogger,
		mapping.OptionUserRealmsToConfirmCreateSessionRealms(userRealms),
	)

	serviceAuthToken := authtoken.New(
		dbConnManager,
		storageAuthToken,
		realmRegistry,
		logger,
		mapping.OptionUserRealmsToCreateSessionRealms(userRealms, jwtConfig),
	)

	useCaseOpenSession := session.NewOpenSession(
		dbConnManager,
		sessionsrv.NewIssuer(storageSession),
		storageUserActivityStat,
		storageAuthToken,          // openSessionCounter
		storageSessionExcessQueue, // excessQueueProducer
		handler.NewAuthFlow(
			authuser.New(
				dbConnManager,
				storageUser,
				storageUserRealm,
				realmRegistry,
				notifierAPI,
				logger,
			),
		),
		serviceAuthToken,
		storageSecureOperation,
		realmRegistry,
		operationLogger,
		logger,
		mapping.OptionUserRealmsToSessionLimitRealms(userRealms),
		int(sessionSoftThreshold),
		int(sessionHardThreshold),
	)

	useCaseContinueSession := session.NewContinueSession(
		storageAuthToken,
		serviceAuthToken,
		eventEmitter,
		operationLogger,
		logger,
	)

	useCaseCloseSession := session.NewCloseSession(
		serviceAuthToken,
	)

	useCaseApplySettings := usecaseuser.NewApplySettings(
		storageUser,
		langResolver,
		timeZoneResolver,
	)

	serviceUserInfo := userinfo.New(
		dbConnManager,
		storageUser,
		storageAuth2fa,
		storageUserActivityStat,
		storageUserRealm,
		locationResolver,
	)

	refreshTokenCookie, err := initRefreshTokenCookie(cookieConfig)
	if err != nil {
		return nil, err
	}

	controller := httpv1.NewAuth(
		requestParser,
		responseSender,
		refreshTokenCookie,
		useCaseCreateUser,
		useCaseConfirmAuthUser,
		useCaseConfirmOperation,
		useCaseOpenSession,
		useCaseContinueSession,
		useCaseCloseSession,
		useCaseApplySettings,
		serviceUserInfo,
		realmRegistry,
		bag.NewOperationResponse(debugFunc),
		debugFunc,
	)

	return controller, nil
}

// initRefreshTokenCookie - создаёт cookie с refresh токеном из провалидированных настроек
// (дефолты и проверка комбинации Secure/SameSite - в authcfg.ResolveRefreshCookie).
func initRefreshTokenCookie(cfg authcfg.RefreshCookie) (*bag.RefreshTokenCookie, error) {
	resolved, err := authcfg.ResolveRefreshCookie(cfg)
	if err != nil {
		return nil, err
	}

	return bag.NewRefreshTokenCookie(
		resolved.Name,
		resolved.Domain,
		resolved.Path,
		resolved.Expiry,
		resolved.Secure,
		resolved.SameSite,
	), nil
}
