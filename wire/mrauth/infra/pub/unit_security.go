package pub

import (
	"time"

	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-webcore/mrserver"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/bag/totp"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/bag"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/service"
	"github.com/mondegor/go-components/mrauth/service/check"
	"github.com/mondegor/go-components/mrauth/usecase/security"
	"github.com/mondegor/go-components/mrauth/usecase/security/handler"
	"github.com/mondegor/go-components/mrauth/validate"
	"github.com/mondegor/go-components/mrnotifier"
)

func initSecurityController(
	logger mrlog.Logger,
	dbConnManager mrstorage.DBConnManager,
	storageUser *repository.UserPostgres,
	storageCheckUser *repository.CheckUserPostgres,
	storageUserRealm *repository.UserRealmPostgres,
	storageAuth2fa *repository.Auth2faPostgres,
	storageSecureOperation *repository.SecureOperationPostgres,
	requestParser *validate.Parser,
	responseFileSender mrserver.FileResponseSender,
	notifierAPI mrnotifier.NoteProducer,
	debugFunc func(value any) string,
) (mrserver.HttpController, error) {
	checkUserService := check.NewUserLogin(
		storageCheckUser,
		storageUserRealm,
	)

	totpAuthenticator := totp.NewAuthenticator("PrintShopApp", 64) // TODO: сделать настройку

	factoryConfirm2FA := service.NewFactoryConfirm2FA(
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

	useCaseChangeEmailProperty := security.NewChangeEmailProperty(
		dbConnManager,
		storageSecureOperation,
		checkUserService,
		notifierAPI,
		factoryConfirm2FA,
		unit.NewChangeEmail(
			crypt.NewSecretGenerator(64), // for tokens
			crypt.NewSecretGenerator(6),
			action.WithMaxAttempts(5), // TODO: в настройки
			action.WithExpiry(30*time.Minute),
		),
	)

	useCaseChangePhoneProperty := security.NewChangePhoneProperty(
		dbConnManager,
		storageSecureOperation,
		checkUserService,
		notifierAPI,
		factoryConfirm2FA,
		unit.NewChangePhone(
			crypt.NewSecretGenerator(64), // for tokens
			crypt.NewSecretGenerator(6),
			action.WithMaxAttempts(5), // TODO: в настройки
			action.WithExpiry(30*time.Minute),
		),
	)

	useCaseChangePasswordProperty := security.NewChangePasswordProperty(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		factoryConfirm2FA,
		unit.NewChangePassword(
			crypt.NewSecretGenerator(64), // for tokens
			crypt.NewSecretGenerator(6),
			action.WithMaxAttempts(5), // TODO: в настройки
			action.WithExpiry(30*time.Minute),
		),
	)

	useCaseChangeTOTPProperty := security.NewChangeTOTPGeneratorProperty(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		factoryConfirm2FA,
		unit.NewChangeTOTP(
			crypt.NewSecretGenerator(64), // for tokens
			crypt.NewSecretGenerator(6),
			totpAuthenticator,
			action.WithMaxAttempts(5), // TODO: в настройки
			action.WithExpiry(30*time.Minute),
		),
	)

	useCaseDisable2FA := security.NewDisable2FA(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		factoryConfirm2FA,
		unit.NewDisable2FA(
			crypt.NewSecretGenerator(64), // for tokens
			crypt.NewSecretGenerator(6),
			action.WithMaxAttempts(5), // TODO: в настройки
			action.WithExpiry(30*time.Minute),
		),
	)

	useCaseApplyOperation := security.NewApplyOperation(
		dbConnManager,
		storageSecureOperation,
		map[string]mrauth.OperationHandler{
			unit.NameConfirmChangeEmail: handler.NewChangeEmail(
				dbConnManager,
				storageUser,
				notifierAPI,
			),
			unit.NameConfirmChangePhone: handler.NewChangePhone(
				dbConnManager,
				storageUser,
				notifierAPI,
			),
			unit.NameConfirmChangePassword: handler.NewChangePassword(
				storageAuth2fa,
				notifierAPI,
				logger,
			),
			unit.NameConfirmDisable2FA: handler.NewDisable2FA(
				dbConnManager,
				storageAuth2fa,
				notifierAPI,
			),
		},
	)

	useCaseRenderTOTPGeneratorQR := security.NewRenderTOTPGeneratorQR(
		storageSecureOperation,
		totpAuthenticator,
	)

	useCaseApplyTOTPGenerator := security.NewApplyTOTPGenerator(
		dbConnManager,
		storageAuth2fa,
		storageSecureOperation,
		crypt.NewSecretGenerator(17), // TODO: в настройки
		totpAuthenticator,
		notifierAPI,
		10, // TODO: в настройки - recovery count
	)

	controller := httpv1.NewSecurity(
		requestParser,
		responseFileSender,
		useCaseChangeEmailProperty,
		useCaseChangePhoneProperty,
		useCaseChangePasswordProperty,
		useCaseApplyOperation,
		useCaseChangeTOTPProperty,
		useCaseRenderTOTPGeneratorQR,
		useCaseApplyTOTPGenerator,
		useCaseDisable2FA,
		bag.NewOperationResponse(debugFunc),
	)

	return controller, nil
}
