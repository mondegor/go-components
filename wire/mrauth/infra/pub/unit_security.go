package pub

import (
	"time"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-webcore/mrserver"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/bag"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	action2 "github.com/mondegor/go-components/mrauth/model/secureoperation/unit/action"
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

	factoryConfirm2FA := service.NewFactoryConfirm2FA(
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
	)

	useCaseChangeEmailProperty := security.NewChangeEmailProperty(
		dbConnManager,
		storageSecureOperation,
		checkUserService,
		notifierAPI,
		factoryConfirm2FA,
		unit.NewChangeEmail(
			crypt.NewTokenGenerator(64),
			crypt.NewCodeGenerator(6),
			action2.WithMaxAttempts(5), // TODO: в настройки
			action2.WithExpiry(30*time.Minute),
		),
	)

	useCaseChangePhoneProperty := security.NewChangePhoneProperty(
		dbConnManager,
		storageSecureOperation,
		checkUserService,
		notifierAPI,
		factoryConfirm2FA,
		unit.NewChangePhone(
			crypt.NewTokenGenerator(64),
			crypt.NewCodeGenerator(6),
			action2.WithMaxAttempts(5), // TODO: в настройки
			action2.WithExpiry(30*time.Minute),
		),
	)

	useCaseChangePasswordProperty := security.NewChangePasswordProperty(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		factoryConfirm2FA,
		unit.NewChangePassword(
			crypt.NewTokenGenerator(64),
			crypt.NewCodeGenerator(6),
			action2.WithMaxAttempts(5), // TODO: в настройки
			action2.WithExpiry(30*time.Minute),
		),
	)

	useCaseChangeTOTPProperty := security.NewChangeTOTPGeneratorProperty(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		factoryConfirm2FA,
		unit.NewChangeTOTP(
			crypt.NewTokenGenerator(64),
			crypt.NewCodeGenerator(6),
			action2.WithMaxAttempts(5), // TODO: в настройки
			action2.WithExpiry(30*time.Minute),
		),
	)

	useCaseDisable2FA := security.NewDisable2FA(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		factoryConfirm2FA,
		unit.NewDisable2FA(
			crypt.NewTokenGenerator(64),
			crypt.NewCodeGenerator(6),
			action2.WithMaxAttempts(5), // TODO: в настройки
			action2.WithExpiry(30*time.Minute),
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

	useCaseApplyTOTPGenerator := security.NewApplyTOTPGenerator(
		dbConnManager,
		storageAuth2fa,
		storageSecureOperation,
		notifierAPI,
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
		bag.NewOperationResponse(debugFunc),
	)

	return controller, nil
}
