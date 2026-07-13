package pub

import (
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/bag/totp"
	"github.com/mondegor/go-components/mrauth/component/produce"
	"github.com/mondegor/go-components/mrauth/component/secureoperation"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/service/auth2fa"
	"github.com/mondegor/go-components/mrauth/usecase/operation"
	"github.com/mondegor/go-components/mrnotifier"
	authcfg "github.com/mondegor/go-components/wire/mrauth/config"
)

func initConfirmOperationUseCase(
	dbConnManager mrstorage.DBConnManager,
	storageSecureOperation *repository.SecureOperationPostgres,
	storageAuth2fa *repository.Auth2FAPostgres,
	notifierAPI mrnotifier.NoteProducer,
	operationLogger *produce.SecureOperationLogger,
	operationConfig authcfg.OperationConfirm,
	auth2faConfig authcfg.Auth2FA,
) *operation.ConfirmOperation {
	recoveryCodeLength := int(auth2faConfig.RecoveryCodeLength)

	return operation.NewConfirmOperation(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		secureoperation.NewConfirmCode(
			crypt.NewSecretGenerator(int(operationConfig.TokenLength)), // TODO: длина должна зависеть от realm
			crypt.NewSecretGenerator(int(operationConfig.CodeLength)),
			auth2fa.NewVerifier(
				storageAuth2fa,
				crypt.NewSecretGenerator(recoveryCodeLength), // длина для генератора неважна: используется только сравнение
				totp.NewAuthenticator("PrintShopApp", 64),
				// аварийный код имеет фиксированную длину recoveryCodeLength - сужаем окно для дешёвой отбраковки
				auth2fa.WithRecoveryCodeLength(recoveryCodeLength, recoveryCodeLength),
				auth2fa.WithRecoveryAlerter(
					auth2fa.NewRecoveryAlerter(notifierAPI, int(auth2faConfig.RecoveryLowThreshold)),
				),
			),
		),
		operationLogger,
	)
}
