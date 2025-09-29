package operation

import (
	"context"
	"time"

	"github.com/mondegor/go-sysmess/mrerr/mr"

	core "github.com/mondegor/go-components/internal"
	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// RevokeOperation - компонент для извлечения настроек, которые хранятся в хранилище данных.
	RevokeOperation struct {
		storageOperation mrauth.SecureOperationStorage
		errorWrapper     core.UseCaseErrorWrapper
	}
)

// NewRevokeOperation - создаёт объект NewRevokeOperation.
func NewRevokeOperation(
	storageOperation mrauth.SecureOperationStorage,
) *RevokeOperation {
	return &RevokeOperation{
		storageOperation: storageOperation,
		errorWrapper:     core.NewUseCaseErrorWrapper(entity.ModelNameSecureOperation),
	}
}

// Perform - comments method.
func (co *RevokeOperation) Perform(ctx context.Context, operationToken string) error {
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
