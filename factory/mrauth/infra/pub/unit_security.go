package pub

import (
	"time"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-webcore/mrserver"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/component/secureoperation"
	"github.com/mondegor/go-components/mrauth/component/secureoperation/action"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/bag"
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
	useCaseErrorWrapper mrerr.UseCaseErrorWrapper,
	serviceErrorWrapper mrerr.ErrorWrapper,
	dbConnManager mrstorage.DBConnManager,
	storageUser *repository.UserPostgres,
	storageCheckUser *repository.CheckUserPostgres,
	storageUserRealm *repository.UserRealmPostgres,
	storageAuth2fa *repository.Auth2faPostgres,
	storageSecureOperation *repository.SecureOperationPostgres,
	requestParser *validate.Parser,
	responseFileSender mrserver.FileResponseSender,
	notifierAPI mrnotifier.NoticeProducer,
	withDebugInfo bool,
) (mrserver.HttpController, error) {
	checkUserService := check.NewUserLogin(
		storageCheckUser,
		storageUserRealm,
		serviceErrorWrapper,
	)

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
		useCaseErrorWrapper,
	)

	useCaseChangeEmailProperty := security.NewChangeEmailProperty(
		dbConnManager,
		storageSecureOperation,
		checkUserService,
		notifierAPI,
		factoryConfirm2FA,
		secureoperation.NewChangeEmail(
			crypt.NewTokenGenerator(64),
			crypt.NewCodeGenerator(6),
			action.WithMaxAttempts(5), // TODO: в настройки
			action.WithExpiry(30*time.Minute),
		),
		useCaseErrorWrapper,
	)

	useCaseChangePhoneProperty := security.NewChangePhoneProperty(
		dbConnManager,
		storageSecureOperation,
		checkUserService,
		notifierAPI,
		factoryConfirm2FA,
		secureoperation.NewChangePhone(
			crypt.NewTokenGenerator(64),
			crypt.NewCodeGenerator(6),
			action.WithMaxAttempts(5), // TODO: в настройки
			action.WithExpiry(30*time.Minute),
		),
		useCaseErrorWrapper,
	)

	useCaseChangePasswordProperty := security.NewChangePasswordProperty(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		factoryConfirm2FA,
		secureoperation.NewChangePassword(
			crypt.NewTokenGenerator(64),
			crypt.NewCodeGenerator(6),
			action.WithMaxAttempts(5), // TODO: в настройки
			action.WithExpiry(30*time.Minute),
		),
		useCaseErrorWrapper,
	)

	useCaseChangeTOTPProperty := security.NewChangeTOTPGeneratorProperty(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		factoryConfirm2FA,
		secureoperation.NewChangeTOTP(
			crypt.NewTokenGenerator(64),
			crypt.NewCodeGenerator(6),
			action.WithMaxAttempts(5), // TODO: в настройки
			action.WithExpiry(30*time.Minute),
		),
		useCaseErrorWrapper,
	)

	useCaseDisable2FA := security.NewDisable2FA(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		factoryConfirm2FA,
		secureoperation.NewDisable2FA(
			crypt.NewTokenGenerator(64),
			crypt.NewCodeGenerator(6),
			action.WithMaxAttempts(5), // TODO: в настройки
			action.WithExpiry(30*time.Minute),
		),
		useCaseErrorWrapper,
	)

	useCaseApplyOperation := security.NewApplyOperation(
		dbConnManager,
		storageSecureOperation,
		useCaseErrorWrapper,
		map[string]mrauth.OperationHandler{
			secureoperation.NameConfirmChangeEmail: handler.NewChangeEmail(
				dbConnManager,
				storageUser,
				notifierAPI,
				useCaseErrorWrapper,
			),
			secureoperation.NameConfirmChangePhone: handler.NewChangePhone(
				dbConnManager,
				storageUser,
				notifierAPI,
				useCaseErrorWrapper,
			),
			secureoperation.NameConfirmChangePassword: handler.NewChangePassword(
				storageAuth2fa,
				notifierAPI,
				useCaseErrorWrapper,
				logger,
			),
			secureoperation.NameConfirmDisable2FA: handler.NewDisable2FA(
				dbConnManager,
				storageAuth2fa,
				notifierAPI,
				useCaseErrorWrapper,
			),
		},
	)

	useCaseApplyTOTPGenerator := security.NewApplyTOTPGenerator(
		dbConnManager,
		storageAuth2fa,
		storageSecureOperation,
		notifierAPI,
		useCaseErrorWrapper,
		"PrintShopApp", // TODO:
	)

	controller := httpv1.NewSecurity(
		requestParser,
		responseFileSender,
		useCaseChangeEmailProperty,
		useCaseChangePhoneProperty,
		useCaseChangePasswordProperty,
		useCaseChangeTOTPProperty,
		useCaseDisable2FA,
		useCaseApplyTOTPGenerator,
		useCaseApplyOperation,
		bag.NewOperationResponse(withDebugInfo),
	)

	return controller, nil
}
