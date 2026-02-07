package clean

import (
	"context"

	"github.com/mondegor/go-sysmess/errors"
)

type (
	// AuthTokenCleaner - объект очищающий очередь от обработанных/сломанных уведомлений.
	AuthTokenCleaner struct {
		storage      authTokenStorage
		errorWrapper errors.Wrapper
	}

	authTokenStorage interface {
		DeleteExpired(ctx context.Context, limit int) error
	}
)

// NewAuthTokenCleaner - создаёт объект OperationCleaner.
func NewAuthTokenCleaner(
	storage authTokenStorage,
) *AuthTokenCleaner {
	return &AuthTokenCleaner{
		storage:      storage,
		errorWrapper: errors.NewUseCaseWrapper(),
	}
}

// RemoveExpired - удаляет ограниченный список уведомлений из успешно обработанных.
func (co *AuthTokenCleaner) RemoveExpired(ctx context.Context, limit int) error {
	if err := co.storage.DeleteExpired(ctx, limit); err != nil {
		return co.errorWrapper.Wrap(err)
	}

	return nil
}
