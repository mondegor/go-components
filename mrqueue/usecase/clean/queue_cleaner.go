package clean

import (
	"context"
	"time"

	"github.com/mondegor/go-sysmess/mrargs"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"
	"github.com/mondegor/go-sysmess/mrevent"

	"github.com/mondegor/go-components/mrqueue"
)

const (
	defaultCompletedExpiry = 24 * time.Hour
	defaultBrokenExpiry    = 72 * time.Hour
)

type (
	// QueueCleaner - объект очищающий очередь от обработанных/сломанных элементов.
	QueueCleaner struct {
		storage          mrqueue.Storage
		storageCompleted mrqueue.CompletedStorage // OPTIONAL
		storageBroken    mrqueue.BrokenStorage    // OPTIONAL
		eventEmitter     mrevent.Emitter
		errorWrapper     mrerr.UseCaseErrorWrapper
		completedExpiry  time.Duration
		brokenExpiry     time.Duration
	}
)

// New - создаёт объект QueueCleaner.
func New(
	storage mrqueue.Storage,
	eventEmitter mrevent.Emitter,
	errorWrapper mrerr.UseCaseErrorWrapper,
	opts ...Option,
) *QueueCleaner {
	co := &QueueCleaner{
		storage:         storage,
		eventEmitter:    mrevent.NewSourceEmitter(eventEmitter, "mrqueue.QueueCleaner"),
		errorWrapper:    mrerr.NewUseCaseErrorWrapper(errorWrapper, "mrqueue.QueueCleaner"),
		completedExpiry: defaultCompletedExpiry,
		brokenExpiry:    defaultBrokenExpiry,
	}

	for _, opt := range opts {
		opt(co)
	}

	return co
}

// RemoveItemsWithoutAttempts - удаляет из очереди ограниченный список элементов находящихся
// в статусе RETRY и с нулевым кол-вом попыток в целях разгрузки очереди. Возвращает SettingID элементов, которые были удалены.
func (co *QueueCleaner) RemoveItemsWithoutAttempts(ctx context.Context, limit int) (itemsIDs []uint64, err error) {
	if limit == 0 {
		return nil, mr.ErrUseCaseIncorrectInternalInputData.New("reason", "limit is zero")
	}

	itemsIDs, err = co.storage.DeleteRetryWithoutAttempts(ctx, limit)
	if err != nil {
		return nil, co.errorWrapper.WrapErrorFailed(err)
	}

	co.eventEmitter.Emit(ctx, "RemoveItemsWithoutAttempts", mrargs.Group{"count": len(itemsIDs)})

	return itemsIDs, nil
}

// RemoveCompletedItems - удаляет ограниченный список элементов из успешно обработанных.
// Возвращает SettingID элементов, которые были удалены.
func (co *QueueCleaner) RemoveCompletedItems(ctx context.Context, limit int) (itemsIDs []uint64, err error) {
	if co.storageBroken == nil {
		return nil, nil
	}

	if limit == 0 {
		return nil, mr.ErrUseCaseIncorrectInternalInputData.New("reason", "limit is zero")
	}

	itemsIDs, err = co.storageCompleted.Delete(ctx, co.completedExpiry, limit)
	if err != nil {
		return nil, co.errorWrapper.WrapErrorFailed(err)
	}

	co.eventEmitter.Emit(ctx, "RemoveCompletedItems", mrargs.Group{"count": len(itemsIDs)})

	return itemsIDs, nil
}

// RemoveBrokenItems - удаляет ограниченный список элементов из журнала ошибок.
// Возвращает SettingID элементов, которые были удалены.
func (co *QueueCleaner) RemoveBrokenItems(ctx context.Context, limit int) (itemsIDs []uint64, err error) {
	if co.storageBroken == nil {
		return nil, nil
	}

	if limit == 0 {
		return nil, mr.ErrUseCaseIncorrectInternalInputData.New("reason", "limit is zero")
	}

	itemsIDs, err = co.storageBroken.Delete(ctx, co.brokenExpiry, limit)
	if err != nil {
		return nil, co.errorWrapper.WrapErrorFailed(err)
	}

	co.eventEmitter.Emit(ctx, "RemoveBrokenItems", mrargs.Group{"count": len(itemsIDs)})

	return itemsIDs, nil
}
