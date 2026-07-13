package pub

import (
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-webcore/mrserver"

	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/component/produce"
	"github.com/mondegor/go-components/mrauth/component/secureoperation"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/bag"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/usecase/operation"
	"github.com/mondegor/go-components/mrauth/validate"
	"github.com/mondegor/go-components/mrnotifier"
	authcfg "github.com/mondegor/go-components/wire/mrauth/config"
)

func initOperationController(
	dbConnManager mrstorage.DBConnManager,
	storageSecureOperation *repository.SecureOperationPostgres,
	useCaseConfirmOperation *operation.ConfirmOperation,
	operationLogger *produce.SecureOperationLogger,
	requestParser *validate.Parser,
	responseSender mrserver.ResponseSender,
	notifierAPI mrnotifier.NoteProducer,
	operationConfig authcfg.OperationConfirm,
	debugFunc func(value any) string,
) (mrserver.HttpController, error) {
	useCaseResendConfirmCode := operation.NewResendCode(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		secureoperation.NewResendCode(
			crypt.NewSecretGenerator(int(operationConfig.TokenLength)), // TODO: длина должна зависеть от realm
			crypt.NewSecretGenerator(int(operationConfig.CodeLength)),
		),
		operationLogger,
	)

	useCaseRevokeOperation := operation.NewRevokeOperation(storageSecureOperation, operationLogger)

	controller := httpv1.NewOperation(
		requestParser,
		responseSender,
		useCaseConfirmOperation,
		useCaseResendConfirmCode,
		useCaseRevokeOperation,
		bag.NewOperationResponse(debugFunc),
		debugFunc,
	)

	return controller, nil
}
