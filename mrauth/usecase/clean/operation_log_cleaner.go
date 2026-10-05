package clean

import (
	"context"
	"time"

	"github.com/mondegor/go-core/errors"
)

type (
	// OperationLogCleaner - объект, удаляющий старые записи журнала защищённых операций.
	OperationLogCleaner struct {
		storageLog   OperationLogStorage
		logLifeTime  time.Duration
		errorWrapper errors.Wrapper
	}

	// OperationLogStorage - хранилище журнала защищённых операций для удаления устаревших записей.
	OperationLogStorage interface {
		DeleteBeforeDate(ctx context.Context, datetime time.Time, limit int) (count int, err error)
	}
)

// NewOperationLogCleaner - создаёт объект OperationLogCleaner.
// logLifeTime - срок хранения записей журнала (записи старше удаляются).
func NewOperationLogCleaner(
	storageLog OperationLogStorage,
	logLifeTime time.Duration,
) *OperationLogCleaner {
	return &OperationLogCleaner{
		storageLog:   storageLog,
		logLifeTime:  logLifeTime,
		errorWrapper: errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - удаляет одну пачку устаревших (старше logLifeTime) записей журнала защищённых
// операций (не более limit). Возвращает число удалённых строк - для ItemBatchPlayer
// это сигнал "пачка была полной, есть ещё".
func (co *OperationLogCleaner) Execute(ctx context.Context, limit int) (count int, err error) {
	if limit < 1 {
		return 0, errors.ErrInternalIncorrectInputData.WithDetails("limit is zero or negative")
	}

	count, err = co.storageLog.DeleteBeforeDate(ctx, time.Now().UTC().Add(-co.logLifeTime), limit)
	if err != nil {
		return 0, co.errorWrapper.Wrap(err)
	}

	return count, nil
}
