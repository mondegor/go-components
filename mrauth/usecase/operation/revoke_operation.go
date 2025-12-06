package operation

import (
	"context"
	"time"

	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"

	"github.com/mondegor/go-components/mrauth"
)

type (
	// RevokeOperation - comment struct.
	RevokeOperation struct {
		storageOperation mrauth.SecureOperationStorage
		errorWrapper     mrerr.UseCaseErrorWrapper
	}
)

// NewRevokeOperation - создаёт объект NewRevokeOperation.
func NewRevokeOperation(
	storageOperation mrauth.SecureOperationStorage,
	errorWrapper mrerr.UseCaseErrorWrapper,
) *RevokeOperation {
	return &RevokeOperation{
		storageOperation: storageOperation,
		errorWrapper:     mrerr.NewUseCaseErrorWrapper(errorWrapper, "mrauth.RevokeOperation"),
	}
}

// Execute - comments method.
func (co *RevokeOperation) Execute(ctx context.Context, operationToken string) error {
	if operationToken == "" {
		return mr.ErrUseCaseIncorrectInputData.New("operationToken is empty")
	}

	op, err := co.storageOperation.FetchOne(ctx, operationToken)
	if err != nil {
		return co.errorWrapper.WrapErrorNotFoundOrFailed(err)
	}

	if time.Now().After(op.ExpiresAt) {
		return mrauth.ErrOperationAlreadyExpired.New()
	}

	if err = co.storageOperation.Delete(ctx, operationToken); err != nil {
		return co.errorWrapper.WrapErrorFailed(err)
	}

	// TODO: Add Operation log:op! ????

	return nil
}

// крон для закрытия, удаления токенов операций
