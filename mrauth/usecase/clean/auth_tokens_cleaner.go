package clean

import (
	"context"

	"github.com/mondegor/go-sysmess/mrerr"

	"github.com/mondegor/go-components/mrauth"
)

type (
	// AuthTokenCleaner - объект очищающий очередь от обработанных/сломанных уведомлений.
	AuthTokenCleaner struct {
		storage      mrauth.AuthTokenStorage
		errorWrapper mrerr.UseCaseErrorWrapper
	}
)

// NewAuthTokenCleaner - создаёт объект OperationCleaner.
func NewAuthTokenCleaner(
	storage mrauth.AuthTokenStorage,
	errorWrapper mrerr.UseCaseErrorWrapper,
) *AuthTokenCleaner {
	return &AuthTokenCleaner{
		storage:      storage,
		errorWrapper: mrerr.NewUseCaseErrorWrapper(errorWrapper, "mrauth.AuthTokenCleaner"),
	}
}

// RemoveExpired - удаляет ограниченный список уведомлений из успешно обработанных.
func (co *AuthTokenCleaner) RemoveExpired(ctx context.Context, limit int) error {
	if err := co.storage.DeleteExpired(ctx, limit); err != nil {
		return co.errorWrapper.WrapErrorFailed(err)
	}

	return nil
}
