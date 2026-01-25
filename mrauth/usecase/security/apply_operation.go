package security

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
)

type (
	// ApplyOperation - comment struct.
	ApplyOperation struct {
		txManager        mrstorage.DBTxManager
		storageOperation mrauth.SecureOperationStorage
		errorWrapper     errors.Wrapper
		handlerMap       map[string]mrauth.OperationHandler
	}
)

// NewApplyOperation - создаёт объект ApplyOperation.
func NewApplyOperation(
	txManager mrstorage.DBTxManager,
	storageOperation mrauth.SecureOperationStorage,
	handlerMap map[string]mrauth.OperationHandler,
) *ApplyOperation {
	return &ApplyOperation{
		txManager:        txManager,
		storageOperation: storageOperation,
		errorWrapper:     errors.NewUseCaseWrapper(),
		handlerMap:       handlerMap,
	}
}

// apply_change.go // to service: validate + store
// change_totp.go отдельный метод

// Execute - comments method.
func (uc *ApplyOperation) Execute(ctx context.Context, userID uuid.UUID, operationToken string) error {
	if operationToken == "" {
		return errors.ErrUseCaseEntityNotFound // TODO: ?может ошибку, что параметр некорректен выдавать?
	}

	op, err := uc.storageOperation.FetchOne(ctx, operationToken)
	if err != nil {
		return uc.errorWrapper.Wrap(err)
	}

	if userID == uuid.Nil || userID != op.UserID {
		return errors.ErrUseCaseAccessForbidden
	}

	// TODO: проверить, что пользователь не заблокирован !!!!!!!

	if op.Status != operationstatus.Confirmed {
		return errors.New("operation id not confirmed")
	}

	handler, ok := uc.handlerMap[op.Name]
	if !ok {
		return errors.New("operation name is not supported")
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err = uc.storageOperation.Delete(ctx, op.Token); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		// TODO: Add Operation log:op! ????

		return handler.Execute(ctx, op.UserID, op.Payload)
	})
	if err != nil {
		return uc.errorWrapper.Wrap(err)
	}

	return nil
}
