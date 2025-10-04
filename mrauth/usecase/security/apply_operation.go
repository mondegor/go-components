package security

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum"
)

type (
	// ApplyOperation - компонент для извлечения настроек, которые хранятся в хранилище данных.
	ApplyOperation struct {
		txManager        mrstorage.DBTxManager
		storageOperation mrauth.SecureOperationStorage
		errorWrapper     mrerr.UseCaseErrorWrapper
		handlerMap       map[string]mrauth.OperationHandler
	}
)

// NewApplyOperation - создаёт объект ApplyOperation.
func NewApplyOperation(
	txManager mrstorage.DBTxManager,
	storageOperation mrauth.SecureOperationStorage,
	errorWrapper mrerr.UseCaseErrorWrapper,
	handlerMap map[string]mrauth.OperationHandler,
) *ApplyOperation {
	return &ApplyOperation{
		txManager:        txManager,
		storageOperation: storageOperation,
		errorWrapper:     mrerr.NewUseCaseErrorWrapper(errorWrapper, entity.ModelNameSecureOperation),
		handlerMap:       handlerMap,
	}
}

// apply_change.go // to service: validate + store
// change_totp.go отдельный метод

// Execute - comments method.
func (uc *ApplyOperation) Execute(ctx context.Context, userID uuid.UUID, operationToken string) error {
	if operationToken == "" {
		return mr.ErrUseCaseEntityNotFound.New() // TODO: ?может ошибку, что параметр некорректен выдавать?
	}

	op, err := uc.storageOperation.FetchOne(ctx, operationToken)
	if err != nil {
		return uc.errorWrapper.WrapErrorNotFoundOrFailed(err)
	}

	if userID == uuid.Nil || userID != op.UserID {
		return mr.ErrUseCaseAccessForbidden.New()
	}

	// TODO: проверить, что пользователь не заблокирован !!!!!!!

	if op.Status != enum.OperationStatusConfirmed {
		return errors.New("operation id not confirmed")
	}

	handler, ok := uc.handlerMap[op.Name]
	if !ok {
		return errors.New("operation name is not supported")
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err = uc.storageOperation.Delete(ctx, op.Token); err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		// TODO: Add Operation log:op! ????

		return handler.Execute(ctx, op.UserID, op.Payload)
	})
	if err != nil {
		return uc.errorWrapper.WrapErrorFailed(err)
	}

	return nil
}
