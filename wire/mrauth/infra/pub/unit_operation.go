package pub

import (
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-webcore/mrserver"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/infra/adapter/collect"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1"
	"github.com/mondegor/go-components/mrauth/infra/pub/controller/httpv1/bag"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/service/secureoperation"
	"github.com/mondegor/go-components/mrauth/usecase/operation"
	"github.com/mondegor/go-components/mrauth/validate"
)

func initOperationController(
	dbConnManager mrstorage.DBConnManager,
	storageSecureOperation *repository.SecureOperationPostgres,
	storageSecurityLog *repository.UserSecurityLogPostgres,
	useCaseConfirmOperation *operation.ConfirmOperation,
	operationLogger *collect.SecureOperationLogger,
	requestParser *validate.Parser,
	responseSender mrserver.ResponseSender,
	notifierAPI mrauth.Notifier,
	debugFunc func(value any) string,
) (mrserver.HttpController, error) {
	secretGenerator := crypt.NewSecretGenerator()

	useCaseResendConfirmCode := operation.NewResendCode(
		dbConnManager,
		storageSecureOperation,
		notifierAPI,
		secureoperation.NewResendCode(secretGenerator, secretGenerator),
		operationLogger,
	)

	useCaseRevokeOperation := operation.NewRevokeOperation(
		dbConnManager,
		storageSecureOperation,
		storageSecurityLog,
		operationLogger,
	)

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
