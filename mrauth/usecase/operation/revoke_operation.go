package operation

import (
	"context"

	"github.com/mondegor/go-sysmess/errors"
)

type (
	// RevokeOperation - comment struct.
	RevokeOperation struct {
		storageOperation operationRevoker
		errorWrapper     errors.Wrapper
	}

	operationRevoker interface {
		Delete(ctx context.Context, token string) error
	}
)

// NewRevokeOperation - создаёт объект NewRevokeOperation.
func NewRevokeOperation(
	storageOperation operationRevoker,
) *RevokeOperation {
	return &RevokeOperation{
		storageOperation: storageOperation,
		errorWrapper:     errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - отзывает (удаляет) операцию по её токену.
func (co *RevokeOperation) Execute(ctx context.Context, operationToken string) error {
	if operationToken == "" {
		return errors.ErrIncorrectInputData.New("operationToken is empty")
	}

	if err := co.storageOperation.Delete(ctx, operationToken); err != nil {
		return co.errorWrapper.Wrap(err)
	}

	// TODO: Add Operation log:op! ????

	return nil
}

// крон для закрытия, удаления токенов операций
