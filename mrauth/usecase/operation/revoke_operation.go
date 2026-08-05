package operation

import (
	"context"

	"github.com/google/uuid"
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
		errorWrapper:     errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - проверяет, что операция принадлежит вызывающему, и отзывает (удаляет) её по токену.
// Операция читается перед удалением, чтобы сверить владельца и чтобы в журнал попало,
// что именно было отозвано.
func (co *RevokeOperation) Execute(ctx context.Context, actor dto.ActorMeta, operationToken string) error {
	// поток отзыва только для залогиненных, поэтому анонимный вызывающий - ошибка проводки
	if actor.VisitorID == uuid.Nil {
		return errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	if operationToken == "" {
		return secureoperation.ErrOperationInvalid
	}

	op, err := co.storageOperation.FetchOne(ctx, operationToken)
	if err != nil {
		if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
			return secureoperation.ErrOperationInvalid
		}

		return co.errorWrapper.Wrap(err)
	}

	// отозвать можно только собственную операцию: владение её токеном доступа не даёт
	if actor.VisitorID != op.UserID {
		// обращение к чужой операции: фиксируем блокировку в журнале
		co.logOperation.Log(
			ctx,
			actor.NewOperationLog(
				op.Name, op.FirstActionMethod(), logstatus.Blocked, logreason.AccessForbidden,
			),
		)

		return errors.ErrAccessForbidden
	}

	// операция потребляется по тому же предъявленному токену и без блокировки, взятой выше:
	// конкурентный отзыв мог удалить строку между выборкой и удалением - это тот же
	// «токен больше не действует», а не нарушение инварианта
	if err = co.storageOperation.Delete(ctx, operationToken); err != nil {
		if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
			return secureoperation.ErrOperationInvalid
		}

		return co.errorWrapper.Wrap(err)
	}

	// операция отозвана: фиксируем в журнале
	co.logOperation.Log(
		ctx,
		actor.NewOperationLog(
			op.Name, op.FirstActionMethod(), logstatus.Revoked, logreason.Unspecified,
		),
	)

	return nil
}
