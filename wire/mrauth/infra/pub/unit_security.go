package pub

import (
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-webcore/mrserver"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/bag/totp"
	"github.com/mondegor/go-components/mrauth/component/produce"
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
	authcfg "github.com/mondegor/go-components/wire/mrauth/config"
	"github.com/mondegor/go-components/wire/mrauth/mapping"
)

func initSecurityController(
	dbConnManager mrstorage.DBConnManager,
	storageUser *repository.UserPostgres,
	storageCheckUser *repository.CheckUserPostgres,
	storageUserRealm *repository.UserRealmPostgres,
	storageAuth2fa *repository.Auth2FAPostgres,
	storageSecureOperation *repository.SecureOperationPostgres,
	operationLogger *produce.SecureOperationLogger,
	requestParser *validate.Parser,
	responseFileSender mrserver.FileResponseSender,
	notifierAPI mrnotifier.NoteProducer,
	userRealms []authcfg.UserRealm,
	operationConfig authcfg.OperationConfirm,
	auth2faConfig authcfg.Auth2FA,
	debugFunc func(value any) string,
) (mrserver.HttpController, error) {
	checkUserService := check.NewUserLogin(
		storageCheckUser,
		storageUserRealm,
		mapping.OptionUserRealmsToRealmRegistry(userRealms),
	)

	totpAuthenticator := totp.NewAuthenticator("PrintShopApp", 64) // TODO: сделать настройку

	factoryConfirm2FA := service.NewFactoryConfirm2FA(
		storageUser,
		storageAuth2fa,
		action.NewConfirmBy2fa(
			[]action.Option{
				action.WithMaxAttempts(int16(operationConfig.CodeMaxAttempts)),
				action.WithExpiry(operationConfig.SessionExpiry),
			},
			[]action.Option{
				action.WithMaxAttempts(int16(operationConfig.CodeMaxAttempts)),
				action.WithExpiry(operationConfig.SessionExpiry),
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
			crypt.NewSecretGenerator(int(operationConfig.TokenLength)),
			crypt.NewSecretGenerator(int(operationConfig.CodeLength)),
			action.WithMaxAttempts(int16(operationConfig.CodeMaxAttempts)),
			action.WithExpiry(operationConfig.SessionExpiry),
		),
		operationLogger,
	)

	useCaseChangePhoneProperty := security.NewChangePhoneProperty(
		dbConnManager,
		storageSecureOperation,
		checkUserService,
		notifierAPI,
		factoryConfirm2FA,
		unit.NewChangePhone(
			crypt.NewSecretGenerator(int(operationConfig.TokenLength)),
			crypt.NewSecretGenerator(int(operationConfig.CodeLength)),
			action.WithMaxAttempts(int16(operationConfig.CodeMaxAttempts)),
			action.WithExpiry(operationConfig.SessionExpiry),
		),
		operationLogger,
	)

	useCaseChangePasswordProperty := security.NewChangePasswordProperty(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		factoryConfirm2FA,
		unit.NewChangePassword(
			crypt.NewSecretGenerator(int(operationConfig.TokenLength)),
			crypt.NewSecretGenerator(int(operationConfig.CodeLength)),
			action.WithMaxAttempts(int16(operationConfig.CodeMaxAttempts)),
			action.WithExpiry(operationConfig.SessionExpiry),
		),
		operationLogger,
	)

	useCaseChangeTOTPProperty := security.NewChangeTOTPGeneratorProperty(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		factoryConfirm2FA,
		unit.NewChangeTOTP(
			crypt.NewSecretGenerator(int(operationConfig.TokenLength)),
			crypt.NewSecretGenerator(int(operationConfig.CodeLength)),
			totpAuthenticator,
			action.WithMaxAttempts(int16(operationConfig.CodeMaxAttempts)),
			action.WithExpiry(operationConfig.SessionExpiry),
		),
		operationLogger,
	)

	useCaseDisable2FA := security.NewDisable2FA(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		factoryConfirm2FA,
		unit.NewDisable2FA(
			crypt.NewSecretGenerator(int(operationConfig.TokenLength)),
			crypt.NewSecretGenerator(int(operationConfig.CodeLength)),
			action.WithMaxAttempts(int16(operationConfig.CodeMaxAttempts)),
			action.WithExpiry(operationConfig.SessionExpiry),
		),
		operationLogger,
	)

	useCaseApplyOperation := security.NewApplyOperation(
		dbConnManager,
		storageSecureOperation,
		operationLogger,
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
		crypt.NewSecretGenerator(int(auth2faConfig.RecoveryCodeLength)),
		totpAuthenticator,
		notifierAPI,
		operationLogger,
		int(auth2faConfig.RecoveryCount),
	)

	useCaseApplyPassword := security.NewApplyPassword(
		dbConnManager,
		storageAuth2fa,
		storageSecureOperation,
		crypt.NewSecretGenerator(int(auth2faConfig.RecoveryCodeLength)),
		notifierAPI,
		operationLogger,
		int(auth2faConfig.RecoveryCount),
	)

	useCaseRegenerateRecovery := security.NewRegenerateRecoveryProperty(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		factoryConfirm2FA,
		unit.NewRegenerateRecovery(
			crypt.NewSecretGenerator(int(operationConfig.TokenLength)),
			crypt.NewSecretGenerator(int(operationConfig.CodeLength)),
			action.WithMaxAttempts(int16(operationConfig.CodeMaxAttempts)),
			action.WithExpiry(operationConfig.SessionExpiry),
		),
		operationLogger,
	)

	useCaseApplyRecovery := security.NewApplyRecovery(
		dbConnManager,
		storageAuth2fa,
		storageSecureOperation,
		crypt.NewSecretGenerator(int(auth2faConfig.RecoveryCodeLength)),
		notifierAPI,
		operationLogger,
		int(auth2faConfig.RecoveryCount),
	)

	controller := httpv1.NewSecurity(
		requestParser,
		responseFileSender,
		useCaseChangeEmailProperty,
		useCaseChangePhoneProperty,
		useCaseApplyOperation,
		useCaseChangePasswordProperty,
		useCaseApplyPassword,
		useCaseChangeTOTPProperty,
		useCaseRenderTOTPGeneratorQR,
		useCaseApplyTOTPGenerator,
		useCaseRegenerateRecovery,
		useCaseApplyRecovery,
		useCaseDisable2FA,
		bag.NewOperationResponse(debugFunc),
	)

	return controller, nil
}
