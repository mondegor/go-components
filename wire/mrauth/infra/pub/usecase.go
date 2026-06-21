package pub

import (
	"github.com/mondegor/go-sysmess/mrstorage"

	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/bag/totp"
	"github.com/mondegor/go-components/mrauth/component/secureoperation"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/service/secondfactor"
	"github.com/mondegor/go-components/mrauth/usecase/operation"
	"github.com/mondegor/go-components/mrnotifier"
	auth "github.com/mondegor/go-components/wire/mrauth/config"
)

func initConfirmOperationUseCase(
	dbConnManager mrstorage.DBConnManager,
	storageSecureOperation *repository.SecureOperationPostgres,
	storageAuth2fa *repository.Auth2faPostgres,
	notifierAPI mrnotifier.NoteProducer,
	operationConfirm auth.OperationConfirm,
) *operation.ConfirmOperation {
	return operation.NewConfirmOperation(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		secureoperation.NewConfirmCode(
			crypt.NewSecretGenerator(int(operationConfirm.TokenLength)), // TODO: длина должна зависеть от realm
			crypt.NewSecretGenerator(int(operationConfirm.CodeLength)),
			secondfactor.NewVerifier(storageAuth2fa, crypt.NewSecretGenerator(17), totp.NewAuthenticator("PrintShopApp", 64)),
		),
	)
}
