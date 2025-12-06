package pub

import (
	"time"

	"github.com/mondegor/go-storage/mrlock"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrevent"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-webcore/mrserver"

	auth "github.com/mondegor/go-components/factory/mrauth/config"
	"github.com/mondegor/go-components/factory/mrauth/mapping"
	"github.com/mondegor/go-components/mrauth/bag/contactaddress"
	"github.com/mondegor/go-components/mrauth/component/secureoperation/action"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/bag"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/service"
	"github.com/mondegor/go-components/mrauth/service/check"
	session2 "github.com/mondegor/go-components/mrauth/service/session"
	usecaseauth "github.com/mondegor/go-components/mrauth/usecase/auth"
	"github.com/mondegor/go-components/mrauth/usecase/operation"
	"github.com/mondegor/go-components/mrauth/usecase/session"
	"github.com/mondegor/go-components/mrauth/usecase/session/handler"
	"github.com/mondegor/go-components/mrauth/validate"
	"github.com/mondegor/go-components/mrnotifier"
)

func initUnitAuthController(
	logger mrlog.Logger,
	eventEmitter mrevent.Emitter,
	useCaseErrorWrapper mrerr.UseCaseErrorWrapper,
	serviceErrorWrapper mrerr.ErrorWrapper,
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
	notifierAPI mrnotifier.NoticeProducer,
	withDebugInfo bool,
	userRealms []auth.UserRealm,
	jwtConfig auth.JWT,
) (mrserver.HttpController, error) {
	checkUserService := check.NewUserLogin(
		storageCheckUser,
		storageUserRealm,
		serviceErrorWrapper,
	)

	contactAddressParser := contactaddress.NewParser()

	useCaseCreateUser := usecaseauth.NewCreateUser(
		dbConnManager,
		checkUserService,
		storageSecureOperation,
		notifierAPI,
		locker,
		contactAddressParser,
		useCaseErrorWrapper,
		mapping.OptionUserRealmsToConfirmCreateUserRealms(userRealms),
	)

	useCaseConfirmAuthUser := usecaseauth.NewCreateSession( // ?????????????????????? CreateSession
		dbConnManager,
		checkUserService,
		storageSecureOperation,
		notifierAPI,
		contactAddressParser,
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
			useCaseErrorWrapper,
		),
		useCaseErrorWrapper,
		mapping.OptionUserRealmsToConfirmCreateSessionRealms(userRealms),
	)

	serviceAuthToken := session2.NewAuthToken(
		storageAuthToken,
		serviceErrorWrapper,
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
			useCaseErrorWrapper,
			logger,
		),
		handler.NewBeforeAuthUser(
			storageUser,
			storageUserRealm,
			notifierAPI,
			useCaseErrorWrapper,
			logger,
		),
		serviceAuthToken,
		useCaseErrorWrapper,
	)

	useCaseContinueSession := session.NewContinueSession(
		dbConnManager,
		storageAuthToken,
		serviceAuthToken,
		eventEmitter,
		useCaseErrorWrapper,
		logger,
	)

	useCaseCloseSession := session.NewCloseSession(
		serviceAuthToken,
		useCaseErrorWrapper,
	)

	useCaseUserInfo := usecaseauth.NewUserInfo(
		dbConnManager,
		storageUser,
		storageAuth2fa,
		storageUserActivityStat,
		storageUserRealm,
		useCaseErrorWrapper,
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
		useCaseUserInfo,
		bag.NewOperationResponse(withDebugInfo),
	)

	return controller, nil
}
