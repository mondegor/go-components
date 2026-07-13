package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// ApplyOperation - применяет подтверждённую защищённую операцию через
	// зарегистрированный обработчик и удаляет её.
	ApplyOperation struct {
		txManager        mrstorage.DBTxManager
		storageOperation operationDeleter
		logOperation     operationLogger
		errorWrapper     errors.Wrapper
		handlerMap       map[string]mrauth.OperationHandler
	}

	operationDeleter interface {
		FetchOneForUpdate(ctx context.Context, token string) (row secureoperation.SecureOperation, err error)
		Delete(ctx context.Context, token string) error
	}

	// operationLogger - best-effort продюсер записей журнала защищённых операций.
	operationLogger interface {
		Log(ctx context.Context, entry entity.SecureOperationLog)
	}
)

// NewApplyOperation - создаёт объект ApplyOperation.
func NewApplyOperation(
	txManager mrstorage.DBTxManager,
	storageOperation operationDeleter,
	logOperation operationLogger,
	handlerMap map[string]mrauth.OperationHandler,
) *ApplyOperation {
	return &ApplyOperation{
		txManager:        txManager,
		storageOperation: storageOperation,
		logOperation:     logOperation,
		errorWrapper:     errors.NewServiceRecordNotFoundWrapper(),
		handlerMap:       handlerMap,
	}
}

// Execute - проверяет, что операция подтверждена и принадлежит пользователю, затем
// в одной транзакции удаляет её и выполняет привязанный к ней обработчик.
// Блокировка исключает повторное применение одной операции при конкурентных запросах.
func (uc *ApplyOperation) Execute(ctx context.Context, actor dto.ActorMeta, operationToken string) error {
	if actor.VisitorID == uuid.Nil {
		return errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	if operationToken == "" {
		return errors.ErrRecordNotFound // TODO: возможно, стоит возвращать ошибку о некорректном параметре
	}

	var (
		operationName  string
		actionMethod   confirmmethod.Enum
		failedLogState logState
	)

	err := uc.txManager.Do(ctx, func(ctx context.Context) error {
		op, err := uc.storageOperation.FetchOneForUpdate(ctx, operationToken)
		if err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		operationName = op.Name
		actionMethod = op.FirstActionMethod()

		if actor.VisitorID != op.UserID {
			failedLogState = newLogState(logstatus.Blocked, logreason.AccessForbidden)

			return errors.ErrAccessForbidden
		}

		// TODO: проверить, что пользователь не заблокирован

		handler, ok := uc.handlerMap[op.Name]
		if !ok {
			return errors.New("operation name is not supported") // TODO: оборачивать в пользовательскую ошибку
		}

		if !op.Is(operationstatus.Confirmed) {
			failedLogState = newLogState(logstatus.Blocked, logreason.NotConfirmed)

			return errors.New("operation is not confirmed")
		}

		if err = uc.storageOperation.Delete(ctx, op.Token); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		return handler.Execute(ctx, op.UserID, op.Payload)
	})
	if err != nil {
		if failedLogState.isSet() {
			// обращение к чужой или ещё не подтверждённой операции:
			// фиксируем блокировку в журнале даже при откате транзакции
			uc.logOperation.Log(
				ctx,
				actor.NewOperationLog(
					operationName, actionMethod, failedLogState.status, failedLogState.reason,
				),
			)
		}

		return uc.errorWrapper.Wrap(err)
	}

	// операция применена: фиксируем в журнале (запись вне транзакции)
	uc.logOperation.Log(
		ctx,
		actor.NewOperationLog(
			operationName, actionMethod, logstatus.Applied, logreason.Unspecified,
		),
	)

	return nil
}
