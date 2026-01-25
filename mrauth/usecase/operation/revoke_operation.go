package operation

import (
	"context"
	"time"

	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth"
)

type (
	// RevokeOperation - comment struct.
	RevokeOperation struct {
		storageOperation mrauth.SecureOperationStorage
		errorWrapper     errors.Wrapper
	}
)

// NewRevokeOperation - создаёт объект NewRevokeOperation.
func NewRevokeOperation(
	storageOperation mrauth.SecureOperationStorage,
) *RevokeOperation {
	return &RevokeOperation{
		storageOperation: storageOperation,
		errorWrapper:     errors.NewUseCaseWrapper(),
	}
}

// Execute - comments method.
func (co *RevokeOperation) Execute(ctx context.Context, operationToken string) error {
	if operationToken == "" {
		return errors.ErrUseCaseIncorrectInputData.New("operationToken is empty")
	}

	op, err := co.storageOperation.FetchOne(ctx, operationToken)
	if err != nil {
		return co.errorWrapper.Wrap(err)
	}

	if time.Now().After(op.ExpiresAt) {
		return mrauth.ErrOperationAlreadyExpired
	}

	if err = co.storageOperation.Delete(ctx, operationToken); err != nil {
		return co.errorWrapper.Wrap(err)
	}

	// TODO: Add Operation log:op! ????

	return nil
}

// крон для закрытия, удаления токенов операций
