package clean

import (
	"context"
	"time"

	"github.com/mondegor/go-core/errors"
)

type (
	// OperationCleaner - объект, удаляющий просроченные защищённые операции и старые записи их лога.
	OperationCleaner struct {
		storage      OperationStorage
		storageLog   OperationLogStorage
		logLifeTime  time.Duration
		errorWrapper errors.Wrapper
	}

	// OperationStorage - хранилище защищённых операций для удаления просроченных.
	OperationStorage interface {
		DeleteExpired(ctx context.Context, limit int) (count int, err error)
	}

	// OperationLogStorage - хранилище лога операций для удаления устаревших записей.
	OperationLogStorage interface {
		DeleteBeforeDate(ctx context.Context, datetime time.Time, limit int) (count int, err error)
	}
)

// NewOperationCleaner - создаёт объект OperationCleaner.
// logLifeTime - срок хранения записей лога операций (записи старше удаляются).
func NewOperationCleaner(
	storage OperationStorage,
	storageLog OperationLogStorage,
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
//
// ВНИМАНИЕ: count - это сумма двух источников (операции + записи лога), каждый ограничен limit,
// поэтому за один вызов может быть удалено до 2*limit строк. Для ItemBatchPlayer это не приводит
// к раннему выходу из цикла (count >= max(источников), цикл продолжается пока есть полная пачка
// хотя бы у одного источника), но размывает контракт "limit = размер батча" и ~2x завышает
// итоговый total, эмитируемый ItemBatchPlayer.
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

	// TODO: count = сумма двух источников (до 2*limit) размывает контракт
	// "limit = размер батча" и ~2x завышает total у ItemBatchPlayer; гонять
	// источники (операции / лог) как отдельные ItemBatchPlayer-воркеры.
	return expiredCount + oldLogCount, nil
}
