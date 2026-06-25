package clean

import (
	"context"
	"time"

	"github.com/mondegor/go-sysmess/errors"
)

type (
	// OperationCleaner - объект, удаляющий просроченные защищённые операции и старые записи их лога.
	OperationCleaner struct {
		storage      operationStorage
		storageLog   operationLogStorage
		logLifeTime  time.Duration
		errorWrapper errors.Wrapper
	}

	operationStorage interface {
		DeleteExpired(ctx context.Context, limit int) (count int, err error)
	}

	operationLogStorage interface {
		DeleteBeforeDate(ctx context.Context, datetime time.Time, limit int) (count int, err error)
	}
)

// NewOperationCleaner - создаёт объект OperationCleaner.
// logLifeTime - срок хранения записей лога операций (записи старше удаляются).
func NewOperationCleaner(
	storage operationStorage,
	storageLog operationLogStorage,
	logLifeTime time.Duration,
) *OperationCleaner {
	return &OperationCleaner{
		storage:      storage,
		storageLog:   storageLog,
		logLifeTime:  logLifeTime,
		errorWrapper: errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - удаляет одну пачку просроченных защищённых операций и устаревших (старше logLifeTime)
// записей их лога (до limit каждого вида). Возвращает суммарное число удалённых строк - для
// ItemBatchPlayer это сигнал "пачка была полной, есть ещё".
func (co *OperationCleaner) Execute(ctx context.Context, limit int) (count int, err error) {
	if limit < 1 {
		return 0, errors.ErrInternalIncorrectInputData.WithDetails("limit is zero or negative")
	}

	expiredCount, err := co.storage.DeleteExpired(ctx, limit)
	if err != nil {
		return 0, co.errorWrapper.Wrap(err)
	}

	oldLogCount, err := co.storageLog.DeleteBeforeDate(ctx, time.Now().Add(-co.logLifeTime), limit)
	if err != nil {
		return 0, co.errorWrapper.Wrap(err)
	}

	return expiredCount + oldLogCount, nil
}
