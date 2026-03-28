package operation

import (
	"context"

	"github.com/mondegor/go-sysmess/errors"

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
		errorWrapper:     errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - comments method.
func (co *RevokeOperation) Execute(ctx context.Context, operationToken string) error {
	if operationToken == "" {
		return errors.ErrIncorrectInputData.New("operationToken is empty")
	}

	// TODO: нужно ли выбирать всю запись?
	op, err := co.storageOperation.FetchOne(ctx, operationToken)
	if err != nil {
		return co.errorWrapper.Wrap(err)
	}

	if err = co.storageOperation.Delete(ctx, op.Token); err != nil {
		return co.errorWrapper.Wrap(err)
	}

	// TODO: Add Operation log:op! ????

	return nil
}

// крон для закрытия, удаления токенов операций
