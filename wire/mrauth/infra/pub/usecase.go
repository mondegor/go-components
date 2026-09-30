package pub

import (
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/bag/totp"
	"github.com/mondegor/go-components/mrauth/infra/adapter/collect"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/service/auth2fa"
	"github.com/mondegor/go-components/mrauth/service/notify"
	"github.com/mondegor/go-components/mrauth/service/secureoperation"
	"github.com/mondegor/go-components/mrauth/usecase/operation"
	authcfg "github.com/mondegor/go-components/wire/mrauth/config"
)

func initConfirmOperationUseCase(
	dbConnManager mrstorage.DBConnManager,
	storageSecureOperation *repository.SecureOperationPostgres,
	storageAuth2fa *repository.Auth2FAPostgres,
	storageUser *repository.UserPostgres,
	notifierAPI mrauth.Notifier,
	actorProps *notify.ActorProps,
	operationLogger *collect.SecureOperationLogger,
	auth2faConfig authcfg.Auth2FA,
) *operation.ConfirmOperation {
	recoveryCodeLength := int(auth2faConfig.RecoveryCodeLength)
	secretGenerator := crypt.NewSecretGenerator()

	return operation.NewConfirmOperation(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		secureoperation.NewConfirmCode(
			secretGenerator,
			secretGenerator,
			auth2fa.NewVerifier(
				storageAuth2fa,
				secretGenerator,
				totp.NewAuthenticator(auth2faConfig.TOTPIssuer, 64),
				// аварийный код имеет фиксированную длину recoveryCodeLength - сужаем окно для дешёвой отбраковки
				auth2fa.WithRecoveryCodeLength(recoveryCodeLength, recoveryCodeLength),
				auth2fa.WithRecoveryAlerter(
					// алертер знает только ID пользователя - декоратор подставляет его email
					auth2fa.NewRecoveryAlerter(
						notify.NewUserEmailNotifier(notifierAPI, storageUser),
						actorProps,
						int(auth2faConfig.RecoveryLowThreshold),
					),
				),
			),
		),
		operationLogger,
	)
}
