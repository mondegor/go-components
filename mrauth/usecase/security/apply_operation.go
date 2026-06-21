package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// ApplyOperation - применяет подтверждённую защищённую операцию через
	// зарегистрированный обработчик и удаляет её.
	ApplyOperation struct {
		txManager        mrstorage.DBTxManager
		storageOperation operationDeleter
		errorWrapper     errors.Wrapper
		handlerMap       map[string]mrauth.OperationHandler
	}

	operationDeleter interface {
		FetchOne(ctx context.Context, token string) (row secureoperation.SecureOperation, err error)
		Delete(ctx context.Context, token string) error
	}
)

// NewApplyOperation - создаёт объект ApplyOperation.
func NewApplyOperation(
	txManager mrstorage.DBTxManager,
	storageOperation operationDeleter,
	handlerMap map[string]mrauth.OperationHandler,
) *ApplyOperation {
	return &ApplyOperation{
		txManager:        txManager,
		storageOperation: storageOperation,
		errorWrapper:     errors.NewServiceRecordNotFoundWrapper(),
		handlerMap:       handlerMap,
	}
}

// Execute - проверяет, что операция подтверждена и принадлежит пользователю, затем
// в одной транзакции удаляет её и выполняет привязанный к ней обработчик.
func (uc *ApplyOperation) Execute(ctx context.Context, userID uuid.UUID, operationToken string) error {
	if userID == uuid.Nil {
		return errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	if operationToken == "" {
		return errors.ErrRecordNotFound // TODO: возможно, стоит возвращать ошибку о некорректном параметре
	}

	// TODO: нужна ли здесь общая транзакция (FetchOne <-> Delete)

	op, err := uc.storageOperation.FetchOne(ctx, operationToken)
	if err != nil {
		return uc.errorWrapper.Wrap(err)
	}

	if userID != op.UserID {
		return errors.ErrAccessForbidden
	}

	// TODO: проверить, что пользователь не заблокирован

	if !op.Is(operationstatus.Confirmed) {
		return errors.New("operation is not confirmed")
	}

	handler, ok := uc.handlerMap[op.Name]
	if !ok {
		return errors.New("operation name is not supported")
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err = uc.storageOperation.Delete(ctx, op.Token); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		// TODO: записать операцию в журнал

		return handler.Execute(ctx, op.UserID, op.Payload)
	})
	if err != nil {
		return uc.errorWrapper.Wrap(err)
	}

	return nil
}
