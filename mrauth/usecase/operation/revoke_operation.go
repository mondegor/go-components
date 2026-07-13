package operation

import (
	"context"

	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// RevokeOperation - usecase отзыва (удаления) защищённой операции.
	RevokeOperation struct {
		storageOperation operationRevoker
		logOperation     operationLogger
		errorWrapper     errors.Wrapper
	}

	operationRevoker interface {
		FetchOne(ctx context.Context, token string) (secureoperation.SecureOperation, error)
		Delete(ctx context.Context, token string) error
	}
)

// NewRevokeOperation - создаёт объект NewRevokeOperation.
func NewRevokeOperation(
	storageOperation operationRevoker,
	logOperation operationLogger,
) *RevokeOperation {
	return &RevokeOperation{
		storageOperation: storageOperation,
		logOperation:     logOperation,
		errorWrapper:     errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - отзывает (удаляет) операцию по её токену.
// Операция читается перед удалением, чтобы в журнал попало, что именно было отозвано.
func (co *RevokeOperation) Execute(ctx context.Context, actor dto.ActorMeta, operationToken string) error {
	if operationToken == "" {
		return errors.ErrIncorrectInputData.New("operationToken is empty")
	}

	op, err := co.storageOperation.FetchOne(ctx, operationToken)
	if err != nil {
		return co.errorWrapper.Wrap(err)
	}

	if err = co.storageOperation.Delete(ctx, operationToken); err != nil {
		return co.errorWrapper.Wrap(err)
	}

	// владелец операции известен - он и фиксируется как посетитель
	// (поток отзыва анонимный, в actor приходит uuid.Nil)
	actor = actor.WithVisitor(op.UserID)

	// операция отозвана: фиксируем в журнале
	co.logOperation.Log(
		ctx,
		actor.NewOperationLog(
			op.Name, op.FirstActionMethod(), logstatus.Revoked, logreason.Unspecified,
		),
	)

	return nil
}

// крон для закрытия, удаления токенов операций
