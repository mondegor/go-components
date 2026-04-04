package pub

import (
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-webcore/mrserver"

	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/component/secureoperation"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/bag"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/usecase/operation"
	"github.com/mondegor/go-components/mrauth/validate"
	"github.com/mondegor/go-components/mrnotifier"
	auth "github.com/mondegor/go-components/wire/mrauth/config"
)

func initOperationController(
	dbConnManager mrstorage.DBConnManager,
	storageSecureOperation *repository.SecureOperationPostgres,
	useCaseConfirmOperation *operation.ConfirmOperation,
	requestParser *validate.Parser,
	responseSender mrserver.ResponseSender,
	notifierAPI mrnotifier.NoteProducer,
	operationConfirm auth.OperationConfirm,
	debugFunc func(value any) string,
) (mrserver.HttpController, error) {
	useCaseResendConfirmCode := operation.NewResendCode(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		secureoperation.NewResendCode(
			crypt.NewTokenGenerator(int(operationConfirm.TokenLength)), // DEFAULT
			crypt.NewCodeGenerator(int(operationConfirm.CodeLength)),   // DEFAULT
		),
	)

	controller := httpv1.NewOperation(
		requestParser,
		responseSender,
		useCaseConfirmOperation,
		useCaseResendConfirmCode,
		bag.NewOperationResponse(debugFunc),
		debugFunc,
	)

	return controller, nil
}
