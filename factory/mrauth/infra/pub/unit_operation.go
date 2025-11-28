package pub

import (
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-webcore/mrserver"

	auth "github.com/mondegor/go-components/factory/mrauth/config"
	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/component/secureoperation"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/bag"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/usecase/operation"
	"github.com/mondegor/go-components/mrauth/validate"
	"github.com/mondegor/go-components/mrnotifier"
)

func initOperationController(
	useCaseErrorWrapper mrerr.UseCaseErrorWrapper,
	dbConnManager mrstorage.DBConnManager,
	storageSecureOperation *repository.SecureOperationPostgres,
	useCaseConfirmOperation *operation.ConfirmOperation,
	requestParser *validate.Parser,
	responseSender mrserver.ResponseSender,
	notifierAPI mrnotifier.NoticeProducer,
	withDebugInfo bool,
	operationConfirm auth.OperationConfirm,
) (mrserver.HttpController, error) {
	useCaseResendConfirmCode := operation.NewResendCode(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		secureoperation.NewResendCode(
			crypt.NewTokenGenerator(int(operationConfirm.TokenLength)), // DEFAULT
			crypt.NewCodeGenerator(int(operationConfirm.CodeLength)),   // DEFAULT
		),
		useCaseErrorWrapper,
	)

	controller := httpv1.NewOperation(
		requestParser,
		responseSender,
		useCaseConfirmOperation,
		useCaseResendConfirmCode,
		bag.NewOperationResponse(withDebugInfo),
	)

	return controller, nil
}
