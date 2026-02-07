package pub

import (
	"time"

	"github.com/mondegor/go-storage/mrlock"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrevent"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-webcore/mrserver"

	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/bag"
	action2 "github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/service"
	"github.com/mondegor/go-components/mrauth/service/check"
	session2 "github.com/mondegor/go-components/mrauth/service/session"
	"github.com/mondegor/go-components/mrauth/service/userinfo"
	usecaseauth "github.com/mondegor/go-components/mrauth/usecase/auth"
	"github.com/mondegor/go-components/mrauth/usecase/operation"
	"github.com/mondegor/go-components/mrauth/usecase/session"
	"github.com/mondegor/go-components/mrauth/usecase/session/handler"
	"github.com/mondegor/go-components/mrauth/validate"
	"github.com/mondegor/go-components/mrnotifier"
	auth "github.com/mondegor/go-components/wire/mrauth/config"
	"github.com/mondegor/go-components/wire/mrauth/mapping"
)

func initUnitAuthController(
	logger mrlog.Logger,
	eventEmitter mrevent.Emitter,
	dbConnManager mrstorage.DBConnManager,
	storageUser *repository.UserPostgres,
	storageCheckUser *repository.CheckUserPostgres,
	storageUserRealm *repository.UserRealmPostgres,
	storageAuth2fa *repository.Auth2faPostgres,
	storageUserActivityStat *repository.UserActivityStatPostgres,
	storageAuthToken *repository.AuthTokenPostgres,
	storageSecureOperation *repository.SecureOperationPostgres,
	useCaseConfirmOperation *operation.ConfirmOperation,
	locker mrlock.Locker,
	requestParser *validate.Parser,
	responseSender mrserver.ResponseSender,
	notifierAPI mrnotifier.NoteProducer,
	withDebugInfo bool,
	userRealms []auth.UserRealm,
	jwtConfig auth.JWT,
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

	useCaseConfirmAuthUser := usecaseauth.NewCreateSession( // ?????????????????????? CreateSession
		dbConnManager,
		checkUserService,
		storageSecureOperation,
		notifierAPI,
		service.NewFactoryConfirm2FA(
			storageUser,
			storageAuth2fa,
			action2.NewConfirmBy2fa(
				[]action2.Option{
					action2.WithMaxAttempts(5), // TODO: в настройки
					action2.WithExpiry(30 * time.Minute),
				},
				[]action2.Option{
					action2.WithMaxAttempts(5), // TODO: в настройки
					action2.WithExpiry(30 * time.Minute),
				},
			),
		),
		mapping.OptionUserRealmsToConfirmCreateSessionRealms(userRealms),
	)

	serviceAuthToken := session2.NewAuthToken(
		storageAuthToken,
		logger,
		mapping.OptionUserRealmsToCreateSessionRealms(userRealms, jwtConfig),
	)

	useCaseOpenSession := session.NewOpenSession(
		dbConnManager,
		storageUserActivityStat,
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
	)

	useCaseContinueSession := session.NewContinueSession(
		dbConnManager,
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

	controller := httpv1.NewAuth(
		requestParser,
		responseSender,
		useCaseCreateUser,
		useCaseConfirmAuthUser,
		useCaseConfirmOperation,
		useCaseOpenSession,
		useCaseContinueSession,
		useCaseCloseSession,
		serviceUserInfo,
		bag.NewOperationResponse(withDebugInfo),
	)

	return controller, nil
}
