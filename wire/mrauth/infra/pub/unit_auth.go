package pub

import (
	"time"

	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrevent"
	"github.com/mondegor/go-sysmess/mrlock"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-webcore/mrserver"

	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/bag"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/service"
	"github.com/mondegor/go-components/mrauth/service/authtoken"
	"github.com/mondegor/go-components/mrauth/service/check"
	"github.com/mondegor/go-components/mrauth/service/userinfo"
	usecaseauth "github.com/mondegor/go-components/mrauth/usecase/auth"
	"github.com/mondegor/go-components/mrauth/usecase/operation"
	"github.com/mondegor/go-components/mrauth/usecase/session"
	"github.com/mondegor/go-components/mrauth/usecase/session/handler"
	"github.com/mondegor/go-components/mrauth/validate"
	"github.com/mondegor/go-components/mrnotifier"
	authcfg "github.com/mondegor/go-components/wire/mrauth/config"
	"github.com/mondegor/go-components/wire/mrauth/mapping"
)

const (
	defaultCookieName   = "RTID"
	defaultCookiePath   = "/"
	defaultCookieExpiry = 180 * 24 * time.Hour
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
	storageSecureOperation *repository.SecureOperationPostgres,
	useCaseConfirmOperation *operation.ConfirmOperation,
	locker mrlock.Locker,
	requestParser *validate.Parser,
	responseSender mrserver.ResponseSender,
	notifierAPI mrnotifier.NoteProducer,
	userRealms []authcfg.UserRealm,
	jwtConfig authcfg.JWT,
	cookieConfig authcfg.RefreshCookie,
	debugFunc func(value any) string,
) (mrserver.HttpController, error) {
	checkUserService := check.NewUserLogin(
		storageCheckUser,
		storageUserRealm,
	)

	useCaseCreateUser := usecaseauth.NewCreateUser(
		dbConnManager,
		checkUserService,
		storageSecureOperation,
		notifierAPI,
		locker,
		mapping.OptionUserRealmsToConfirmCreateUserRealms(userRealms),
	)

	useCaseConfirmAuthUser := usecaseauth.NewCreateSession(
		dbConnManager,
		checkUserService,
		storageSecureOperation,
		notifierAPI,
		service.NewFactoryConfirm2FA(
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
		),
		mapping.OptionUserRealmsToConfirmCreateSessionRealms(userRealms),
	)

	serviceAuthToken := authtoken.New(
		dbConnManager,
		storageAuthToken,
		logger,
		mapping.OptionUserRealmsToCreateSessionRealms(userRealms, jwtConfig),
	)

	useCaseOpenSession := session.NewOpenSession(
		dbConnManager,
		storageSession,
		storageUserActivityStat,
		storageAuthToken, // openSessionFetcher
		storageAuthToken, // sessionCloser
		locker,
		handler.NewCreateUser(
			dbConnManager,
			storageUser,
			storageUserRealm,
			notifierAPI,
			logger,
		),
		handler.NewBeforeAuthUser(
			storageUser,
			storageUserRealm,
			notifierAPI,
			logger,
		),
		serviceAuthToken,
		mapping.OptionUserRealmsToSessionLimitRealms(userRealms),
	)

	useCaseContinueSession := session.NewContinueSession(
		storageAuthToken,
		serviceAuthToken,
		eventEmitter,
		logger,
	)

	useCaseCloseSession := session.NewCloseSession(
		serviceAuthToken,
	)

	serviceUserInfo := userinfo.New(
		dbConnManager,
		storageUser,
		storageAuth2fa,
		storageUserActivityStat,
		storageUserRealm,
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
		serviceUserInfo,
		bag.NewOperationResponse(debugFunc),
		debugFunc,
	)

	return controller, nil
}

// initRefreshTokenCookie - создаёт cookie с refresh токеном, подставляя значения по умолчанию
// для незаданных полей конфига (Domain обязан задать host-приложение, иначе возвращается ошибка).
func initRefreshTokenCookie(cfg authcfg.RefreshCookie) (*bag.RefreshTokenCookie, error) {
	if cfg.Domain == "" {
		return nil, errors.New("refresh token cookie domain is required")
	}

	if cfg.Name == "" {
		cfg.Name = defaultCookieName
	}

	if cfg.Path == "" {
		cfg.Path = defaultCookiePath
	}

	if cfg.Expiry == 0 {
		cfg.Expiry = defaultCookieExpiry
	}

	return bag.NewRefreshTokenCookie(cfg.Name, cfg.Domain, cfg.Path, cfg.Expiry), nil
}
