package clean

import (
	"context"

	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth"
)

type (
	// AuthTokenCleaner - объект очищающий очередь от обработанных/сломанных уведомлений.
	AuthTokenCleaner struct {
		storage      mrauth.AuthTokenStorage
		errorWrapper errors.Wrapper
	}
)

// NewAuthTokenCleaner - создаёт объект OperationCleaner.
func NewAuthTokenCleaner(
	storage mrauth.AuthTokenStorage,
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
