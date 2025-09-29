package clean

import (
	"context"

	core "github.com/mondegor/go-components/internal"
	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// AuthTokenCleaner - объект очищающий очередь от обработанных/сломанных уведомлений.
	AuthTokenCleaner struct {
		storage      mrauth.AuthTokenStorage
		errorWrapper core.UseCaseErrorWrapper
	}
)

// NewAuthTokenCleaner - создаёт объект OperationCleaner.
func NewAuthTokenCleaner(storage mrauth.AuthTokenStorage) *AuthTokenCleaner {
	return &AuthTokenCleaner{
		storage:      storage,
		errorWrapper: core.NewUseCaseErrorWrapper(entity.ModelNameAuthToken),
	}
}

// RemoveExpired - удаляет ограниченный список уведомлений из успешно обработанных.
func (co *AuthTokenCleaner) RemoveExpired(ctx context.Context, limit int) error {
	if err := co.storage.DeleteExpired(ctx, limit); err != nil {
		return co.errorWrapper.WrapErrorFailed(err)
	}

	return nil
}
