package pub

import (
	"time"

	"github.com/mondegor/go-core/mrevent"
	"github.com/mondegor/go-core/mrlock"
	"github.com/mondegor/go-core/mrlog"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-webcore/mrserver"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/infra/adapter/collect"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/bag"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/service"
	"github.com/mondegor/go-components/mrauth/service/authtoken"
	"github.com/mondegor/go-components/mrauth/service/authuser"
	"github.com/mondegor/go-components/mrauth/service/check"
	"github.com/mondegor/go-components/mrauth/service/notify"
	"github.com/mondegor/go-components/mrauth/service/secureoperation"
	sessionsrv "github.com/mondegor/go-components/mrauth/service/session"
	"github.com/mondegor/go-components/mrauth/service/userinfo"
	usecaseauth "github.com/mondegor/go-components/mrauth/usecase/auth"
	"github.com/mondegor/go-components/mrauth/usecase/operation"
	"github.com/mondegor/go-components/mrauth/usecase/session"
	"github.com/mondegor/go-components/mrauth/usecase/session/handler"
	usecaseuser "github.com/mondegor/go-components/mrauth/usecase/user"
	"github.com/mondegor/go-components/mrauth/validate"
	authcfg "github.com/mondegor/go-components/wire/mrauth/config"
	"github.com/mondegor/go-components/wire/mrauth/mapping"
)

func initUnitAuthController(
	logger mrlog.Logger,
	eventEmitter mrevent.Emitter,
	dbConnManager mrstorage.DBConnManager,
	operationOpener *secureoperation.Opener,
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
	operationLogger *collect.SecureOperationLogger,
	locker mrlock.Locker,
	requestParser *validate.Parser,
	responseSender mrserver.ResponseSender,
	notifierAPI mrauth.Notifier,
	actorProps *notify.ActorProps,
	userRealms []authcfg.UserRealm,
	auth2faConfig authcfg.Auth2FA,
	jwtConfig authcfg.JWT,
	cookieConfig authcfg.RefreshCookie,
	sessionSoftThreshold, sessionHardThreshold int8,
	sessionLimitRetryAfter time.Duration,
	debugFunc func(value any) string,
	locationResolver mrauth.LocationResolver,
) (mrserver.HttpController, error) {
	realmRegistry := mapping.OptionUserRealmsToRealmRegistry(userRealms)

	// подставной второй фактор для аккаунтов без 2FA: вход по аварийному коду обязан выглядеть
	// одинаково при любом состоянии аккаунта (см. unit.AuthorizeUserByRecovery)
	decoyFactorSelector, err := crypt.NewDecoyFactorSelector(
		[]byte(auth2faConfig.DecoyFactorSalt),
		uint32(auth2faConfig.DecoyTOTPPercent),
	)
	if err != nil {
		return nil, err
	}

	checkUserService := check.NewUserLogin(
		storageCheckUser,
		storageUserRealm,
		realmRegistry,
	)

	confirm2faOpts := []action.Option{
		action.WithMaxAttempts(int16(auth2faConfig.ConfirmMaxAttempts)),
		action.WithExpiry(auth2faConfig.ConfirmExpiry),
	}

	factory2FA := service.NewFactoryConfirm2FA(
		storageUser,
		storageAuth2fa,
		action.NewConfirmBy2fa(confirm2faOpts, confirm2faOpts),
	)

	useCaseCreateUser := usecaseauth.NewCreateUser(
		operationOpener,
		checkUserService,
		factory2FA,
		locker,
		operationLogger,
		mapping.OptionUserRealmsToConfirmCreateRealmUsers(userRealms),
	)

	useCaseConfirmAuthUser := usecaseauth.NewCreateSession(
		operationOpener,
		checkUserService,
		factory2FA,
		operationLogger,
		mapping.OptionUserRealmsToConfirmCreateSessionRealms(userRealms),
	)

	createSessionByRecoveryRealms := mapping.OptionUserRealmsToConfirmCreateSessionByRecoveryRealms(
		userRealms,
		decoyFactorSelector,
		confirm2faOpts,
	)

	useCaseConfirmAuthUserByRecovery := usecaseauth.NewCreateSessionByRecovery(
		operationOpener,
		checkUserService,
		factory2FA,
		operationLogger,
		createSessionByRecoveryRealms,
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
				actorProps,
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

	useCaseChangeSettings := usecaseuser.NewChangeSettings(
		dbConnManager,
		storageUser,
		storageAuthToken,
	)

	serviceUserInfo := userinfo.New(
		dbConnManager,
		storageUser,
		storageAuth2fa,
		storageUserActivityStat,
		storageUserRealm,
		storageSecureOperation,
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
		useCaseConfirmAuthUserByRecovery,
		useCaseConfirmOperation,
		useCaseOpenSession,
		useCaseContinueSession,
		useCaseCloseSession,
		useCaseChangeSettings,
		serviceUserInfo,
		realmRegistry,
		bag.NewOperationResponse(debugFunc),
		sessionLimitRetryAfter,
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
