package clean

import (
	"context"
	"time"

	core "github.com/mondegor/go-components/internal"
	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// OperationCleaner - объект очищающий очередь от обработанных/сломанных уведомлений.
	OperationCleaner struct {
		storage      mrauth.SecureOperationStorage
		storageLog   mrauth.SecureOperationLogStorage
		errorWrapper core.UseCaseErrorWrapper
	}
)

// NewOperationCleaner - создаёт объект OperationCleaner.
func NewOperationCleaner(storage mrauth.SecureOperationStorage, storageLog mrauth.SecureOperationLogStorage) *OperationCleaner {
	return &OperationCleaner{
		storage:      storage,
		storageLog:   storageLog,
		errorWrapper: core.NewUseCaseErrorWrapper(entity.ModelNameSecureOperation),
	}
}

// RemoveExpired - удаляет ограниченный список уведомлений из успешно обработанных.
func (co *OperationCleaner) RemoveExpired(ctx context.Context, limit int) error {
	if err := co.storage.DeleteExpired(ctx, limit); err != nil {
		return co.errorWrapper.WrapErrorFailed(err)
	}

	return nil
}

// RemoveOldLog - удаляет ограниченный список уведомлений из успешно обработанных.
func (co *OperationCleaner) RemoveOldLog(ctx context.Context, logLifeTime time.Duration, limit int) error {
	if err := co.storageLog.DeleteBeforeDate(ctx, time.Now().Add(-logLifeTime), limit); err != nil {
		return co.errorWrapper.WrapErrorFailed(err)
	}

	return nil
}
