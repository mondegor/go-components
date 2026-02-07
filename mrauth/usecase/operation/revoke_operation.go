package operation

import (
	"context"
	"time"

	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

type (
	// RevokeOperation - comment struct.
	RevokeOperation struct {
		storageOperation operationRevoker
		errorWrapper     errors.Wrapper
	}

	operationRevoker interface {
		FetchOne(ctx context.Context, token string) (row secureoperation.SecureOperation, err error)
		Delete(ctx context.Context, token string) error
	}
)

// NewRevokeOperation - создаёт объект NewRevokeOperation.
func NewRevokeOperation(
	storageOperation operationRevoker,
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
