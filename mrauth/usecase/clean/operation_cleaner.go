package clean

import (
	"context"
	"time"

	"github.com/mondegor/go-sysmess/errors"
)

type (
	// OperationCleaner - объект очищающий очередь от обработанных/сломанных уведомлений.
	OperationCleaner struct {
		storage      operationStorage
		storageLog   operationLogStorage
		errorWrapper errors.Wrapper
	}

	operationStorage interface {
		DeleteExpired(ctx context.Context, limit int) error
	}

	operationLogStorage interface {
		DeleteBeforeDate(ctx context.Context, datetime time.Time, limit int) error
	}
)

// NewOperationCleaner - создаёт объект OperationCleaner.
func NewOperationCleaner(
	storage operationStorage,
	storageLog operationLogStorage,
) *OperationCleaner {
	return &OperationCleaner{
		storage:      storage,
		storageLog:   storageLog,
		errorWrapper: errors.NewUseCaseWrapper(),
	}
}

// RemoveExpired - удаляет ограниченный список уведомлений из успешно обработанных.
func (co *OperationCleaner) RemoveExpired(ctx context.Context, limit int) error {
	if err := co.storage.DeleteExpired(ctx, limit); err != nil {
		return co.errorWrapper.Wrap(err)
	}

	return nil
}

// RemoveOldLog - удаляет ограниченный список уведомлений из успешно обработанных.
func (co *OperationCleaner) RemoveOldLog(ctx context.Context, logLifeTime time.Duration, limit int) error {
	if err := co.storageLog.DeleteBeforeDate(ctx, time.Now().Add(-logLifeTime), limit); err != nil {
		return co.errorWrapper.Wrap(err)
	}

	return nil
}
