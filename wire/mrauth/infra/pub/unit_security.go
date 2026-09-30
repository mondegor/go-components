package pub

import (
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-webcore/mrserver"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/bag/totp"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/infra/adapter/collect"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/bag"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/service"
	"github.com/mondegor/go-components/mrauth/service/check"
	"github.com/mondegor/go-components/mrauth/service/notify"
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
	operationLogger *collect.SecureOperationLogger,
	requestParser *validate.Parser,
	responseFileSender mrserver.FileResponseSender,
	notifierAPI mrauth.Notifier,
	actorProps *notify.ActorProps,
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

	// токены, коды и аварийные коды: длину каждого секрета задаёт его потребитель
	secretGenerator := crypt.NewSecretGenerator()

	// звено кода с email у операций управления безопасностью аккаунта
	confirmByEmailOpts := []action.Option{
		action.WithCodeLength(int16(operationConfig.SendByEmail.CodeLength)),
		action.WithMaxAttempts(int16(operationConfig.SendByEmail.MaxAttempts)),
		action.WithMaxResends(int16(operationConfig.SendByEmail.MaxResends)),
		action.WithMinResendTime(operationConfig.SendByEmail.MinResendTime),
		action.WithExpiry(operationConfig.SessionExpiry),
	}

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
			secretGenerator,
			int(operationConfig.TokenLength),
			secretGenerator,
			confirmByEmailOpts...,
		),
	)

	useCaseChangeEmailByRecoveryProperty := security.NewChangeEmailByRecoveryProperty(
		operationOpener,
		checkUserService,
		factoryConfirm2FA,
		// аварийный код предъявляется вместо кода с текущего адреса, поэтому его звено
		// настраивается наравне со вторым фактором, а не остаётся на умолчаниях
		unit.NewChangeEmailRequestByRecovery(
			secretGenerator,
			int(operationConfig.TokenLength),
			confirm2faOpts...,
		),
	)

	useCaseApplyEmail := security.NewApplyEmail(
		dbConnManager,
		storageSecureOperation,
		checkUserService,
		unit.NewChangeEmail(
			secretGenerator,
			int(operationConfig.TokenLength),
			secretGenerator,
			operationConfig.NewEmailExpiry,
			confirmByEmailOpts...,
		),
		operationOpener,
		notifierAPI,
		actorProps,
		operationLogger,
	)

	useCaseChangePhoneProperty := security.NewChangePhoneProperty(
		operationOpener,
		checkUserService,
		factoryConfirm2FA,
		unit.NewChangePhone(
			secretGenerator,
			int(operationConfig.TokenLength),
			secretGenerator,
			confirmByEmailOpts...,
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
			secretGenerator,
			int(operationConfig.TokenLength),
			secretGenerator,
			confirmByEmailOpts...,
		),
		passwordService,
	)

	useCaseChangeTOTPProperty := security.NewChangeTOTPGeneratorProperty(
		operationOpener,
		factoryConfirm2FA,
		unit.NewChangeTOTP(
			secretGenerator,
			int(operationConfig.TokenLength),
			secretGenerator,
			totpAuthenticator,
			confirmByEmailOpts...,
		),
	)

	useCaseDisable2FA := security.NewDisable2FA(
		operationOpener,
		factoryConfirm2FA,
		unit.NewDisable2FA(
			secretGenerator,
			int(operationConfig.TokenLength),
			secretGenerator,
			confirmByEmailOpts...,
		),
	)

	useCaseApplyOperation := security.NewApplyOperation(
		dbConnManager,
		storageSecureOperation,
		operationLogger,
		map[operationtype.Enum]mrauth.OperationHandler{
			operationtype.ChangeEmailConfirm: handler.NewChangeEmail(
				dbConnManager,
				storageUser,
				operationRevoker,
				notifierAPI,
				actorProps,
			),
			operationtype.ChangePhone: handler.NewChangePhone(
				dbConnManager,
				storageUser,
				notifierAPI,
				actorProps,
			),
			operationtype.Disable2FA: handler.NewDisable2FA(
				dbConnManager,
				storageAuth2fa,
				operationRevoker,
				notifierAPI,
				actorProps,
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
		secretGenerator,
		totpAuthenticator,
		notifierAPI,
		actorProps,
		operationLogger,
		int(auth2faConfig.RecoveryCount),
		int(auth2faConfig.RecoveryCodeLength),
	)

	useCaseApplyPassword := security.NewApplyPassword(
		dbConnManager,
		storageAuth2fa,
		storageSecureOperation,
		operationRevoker,
		secretGenerator,
		notifierAPI,
		actorProps,
		operationLogger,
		int(auth2faConfig.RecoveryCount),
		int(auth2faConfig.RecoveryCodeLength),
	)

	useCaseRegenerateRecovery := security.NewRegenerateRecoveryProperty(
		operationOpener,
		factoryConfirm2FA,
		unit.NewRegenerateRecovery(
			secretGenerator,
			int(operationConfig.TokenLength),
			secretGenerator,
			confirmByEmailOpts...,
		),
	)

	useCaseApplyRecovery := security.NewApplyRecovery(
		dbConnManager,
		storageAuth2fa,
		storageSecureOperation,
		secretGenerator,
		notifierAPI,
		actorProps,
		operationLogger,
		int(auth2faConfig.RecoveryCount),
		int(auth2faConfig.RecoveryCodeLength),
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
