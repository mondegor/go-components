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
	"github.com/mondegor/go-components/mrauth/service/secureoperation"
	"github.com/mondegor/go-components/mrauth/usecase/security"
	"github.com/mondegor/go-components/mrauth/usecase/security/handler"
	"github.com/mondegor/go-components/mrauth/validate"
	authcfg "github.com/mondegor/go-components/wire/mrauth/config"
	"github.com/mondegor/go-components/wire/mrauth/mapping"
)

func initSecurityController(
	dbConnManager mrstorage.DBConnManager,
	operationOpener *secureoperation.Opener,
	storageUser *repository.UserPostgres,
	storageCheckUser *repository.CheckUserPostgres,
	storageUserRealm *repository.UserRealmPostgres,
	storageAuth2fa *repository.Auth2FAPostgres,
	storageSecureOperation *repository.SecureOperationPostgres,
	operationLogger *produce.SecureOperationLogger,
	requestParser *validate.Parser,
	responseFileSender mrserver.FileResponseSender,
	notifierAPI mrauth.Notifier,
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

	totpAuthenticator := totp.NewAuthenticator(auth2faConfig.TOTPIssuer, 64)

	// отзыв незавершённых операций пользователя при смене состояния 2FA
	operationRevoker := secureoperation.NewRevoker(storageSecureOperation, operationLogger)

	confirm2faOpts := []action.Option{
		action.WithMaxAttempts(int16(auth2faConfig.ConfirmMaxAttempts)),
		action.WithExpiry(auth2faConfig.ConfirmExpiry),
	}

	factoryConfirm2FA := service.NewFactoryConfirm2FA(
		storageUser,
		storageAuth2fa,
		action.NewConfirmBy2fa(confirm2faOpts, confirm2faOpts),
	)

	useCaseChangeEmailProperty := security.NewChangeEmailProperty(
		operationOpener,
		checkUserService,
		factoryConfirm2FA,
		unit.NewChangeEmailRequest(
			crypt.NewSecretGenerator(int(operationConfig.TokenLength)),
			crypt.NewSecretGenerator(int(operationConfig.CodeLength)),
			action.WithMaxAttempts(int16(operationConfig.CodeMaxAttempts)),
			action.WithExpiry(operationConfig.SessionExpiry),
		),
	)

	useCaseChangeEmailByRecoveryProperty := security.NewChangeEmailByRecoveryProperty(
		operationOpener,
		checkUserService,
		factoryConfirm2FA,
		// аварийный код предъявляется вместо кода с текущего адреса, поэтому его звено
		// настраивается наравне со вторым фактором, а не остаётся на умолчаниях
		unit.NewChangeEmailRequestByRecovery(
			crypt.NewSecretGenerator(int(operationConfig.TokenLength)),
			confirm2faOpts...,
		),
	)

	useCaseApplyEmail := security.NewApplyEmail(
		dbConnManager,
		storageSecureOperation,
		checkUserService,
		unit.NewChangeEmail(
			crypt.NewSecretGenerator(int(operationConfig.TokenLength)),
			crypt.NewSecretGenerator(int(operationConfig.CodeLength)),
			operationConfig.NewEmailExpiry,
			action.WithMaxAttempts(int16(operationConfig.CodeMaxAttempts)),
		),
		operationOpener,
		notifierAPI,
		operationLogger,
	)

	useCaseChangePhoneProperty := security.NewChangePhoneProperty(
		operationOpener,
		checkUserService,
		factoryConfirm2FA,
		unit.NewChangePhone(
			crypt.NewSecretGenerator(int(operationConfig.TokenLength)),
			crypt.NewSecretGenerator(int(operationConfig.CodeLength)),
			action.WithMaxAttempts(int16(operationConfig.CodeMaxAttempts)),
			action.WithExpiry(operationConfig.SessionExpiry),
		),
	)

	passwordService, err := initPasswordService(auth2faConfig)
	if err != nil {
		return nil, err
	}

	useCaseChangePasswordProperty := security.NewChangePasswordProperty(
		operationOpener,
		factoryConfirm2FA,
		unit.NewChangePassword(
			crypt.NewSecretGenerator(int(operationConfig.TokenLength)),
			crypt.NewSecretGenerator(int(operationConfig.CodeLength)),
			action.WithMaxAttempts(int16(operationConfig.CodeMaxAttempts)),
			action.WithExpiry(operationConfig.SessionExpiry),
		),
		passwordService,
	)

	useCaseChangeTOTPProperty := security.NewChangeTOTPGeneratorProperty(
		operationOpener,
		factoryConfirm2FA,
		unit.NewChangeTOTP(
			crypt.NewSecretGenerator(int(operationConfig.TokenLength)),
			crypt.NewSecretGenerator(int(operationConfig.CodeLength)),
			totpAuthenticator,
			action.WithMaxAttempts(int16(operationConfig.CodeMaxAttempts)),
			action.WithExpiry(operationConfig.SessionExpiry),
		),
	)

	useCaseDisable2FA := security.NewDisable2FA(
		operationOpener,
		factoryConfirm2FA,
		unit.NewDisable2FA(
			crypt.NewSecretGenerator(int(operationConfig.TokenLength)),
			crypt.NewSecretGenerator(int(operationConfig.CodeLength)),
			action.WithMaxAttempts(int16(operationConfig.CodeMaxAttempts)),
			action.WithExpiry(operationConfig.SessionExpiry),
		),
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
				operationRevoker,
				notifierAPI,
			),
		},
	)

	useCaseGetTOTPGeneratorSecret := security.NewGetTOTPGeneratorSecret(
		storageSecureOperation,
		totpAuthenticator,
	)

	useCaseRenderTOTPGeneratorQR := security.NewRenderTOTPGeneratorQR(
		storageSecureOperation,
		totpAuthenticator,
	)

	useCaseApplyTOTPGenerator := security.NewApplyTOTPGenerator(
		dbConnManager,
		storageAuth2fa,
		storageSecureOperation,
		operationRevoker,
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
		operationRevoker,
		crypt.NewSecretGenerator(int(auth2faConfig.RecoveryCodeLength)),
		notifierAPI,
		operationLogger,
		int(auth2faConfig.RecoveryCount),
	)

	useCaseRegenerateRecovery := security.NewRegenerateRecoveryProperty(
		operationOpener,
		factoryConfirm2FA,
		unit.NewRegenerateRecovery(
			crypt.NewSecretGenerator(int(operationConfig.TokenLength)),
			crypt.NewSecretGenerator(int(operationConfig.CodeLength)),
			action.WithMaxAttempts(int16(operationConfig.CodeMaxAttempts)),
			action.WithExpiry(operationConfig.SessionExpiry),
		),
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
		useCaseChangeEmailByRecoveryProperty,
		useCaseApplyEmail,
		useCaseChangePhoneProperty,
		useCaseApplyOperation,
		useCaseChangePasswordProperty,
		useCaseApplyPassword,
		useCaseChangeTOTPProperty,
		useCaseGetTOTPGeneratorSecret,
		useCaseRenderTOTPGeneratorQR,
		useCaseApplyTOTPGenerator,
		useCaseRegenerateRecovery,
		useCaseApplyRecovery,
		useCaseDisable2FA,
		bag.NewOperationResponse(debugFunc),
	)

	return controller, nil
}
