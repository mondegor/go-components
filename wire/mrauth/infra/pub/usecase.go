package pub

import (
	"github.com/mondegor/go-sysmess/mrstorage"

	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/component/secureoperation"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/usecase/operation"
	"github.com/mondegor/go-components/mrnotifier"
	auth "github.com/mondegor/go-components/wire/mrauth/config"
)

func initConfirmOperationUseCase(
	dbConnManager mrstorage.DBConnManager,
	storageSecureOperation *repository.SecureOperationPostgres,
	notifierAPI mrnotifier.NoteProducer,
	operationConfirm auth.OperationConfirm,
) *operation.ConfirmOperation {
	return operation.NewConfirmOperation(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		secureoperation.NewConfirmCode(
			crypt.NewTokenGenerator(int(operationConfirm.TokenLength)), // DEFAULT TODO: должно зависить от realm
			crypt.NewCodeGenerator(int(operationConfirm.CodeLength)),   // DEFAULT
		),
	)
}
