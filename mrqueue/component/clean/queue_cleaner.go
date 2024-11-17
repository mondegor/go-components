package clean

import (
	"context"
	"time"

	"github.com/mondegor/go-sysmess/mrmsg"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrsender"
	"github.com/mondegor/go-webcore/mrsender/decorator"

	"github.com/mondegor/go-components/mrqueue"
	"github.com/mondegor/go-components/mrqueue/entity"
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
		eventEmitter     mrsender.EventEmitter
		errorWrapper     mrcore.UseCaseErrorWrapper
		completedExpiry  time.Duration
		brokenExpiry     time.Duration
	}
)

// New - создаёт объект QueueCleaner.
func New(
	storage mrqueue.Storage,
	eventEmitter mrsender.EventEmitter,
	errorWrapper mrcore.UseCaseErrorWrapper,
	opts ...Option,
) *QueueCleaner {
	co := &QueueCleaner{
		storage:         storage,
		eventEmitter:    decorator.NewSourceEmitter(eventEmitter, entity.ModelNameItem),
		errorWrapper:    errorWrapper,
		completedExpiry: defaultCompletedExpiry,
		brokenExpiry:    defaultBrokenExpiry,
	}

	for _, opt := range opts {
		opt(co)
	}

	return co
}

// RemoveItemsWithoutAttempts - удаляет из очереди ограниченный список элементов находящихся
// в статусе RETRY и с нулевым кол-вом попыток в целях разгрузки очереди. Возвращает ID элементов, которые были удалены.
func (co *QueueCleaner) RemoveItemsWithoutAttempts(ctx context.Context, limit uint32) (itemsIDs []uint64, err error) {
	if limit == 0 {
		return nil, mrcore.ErrUseCaseIncorrectInputData.New("limit", "value is zero")
	}

	itemsIDs, err = co.storage.DeleteRetryWithoutAttempts(ctx, limit)
	if err != nil {
		if !mrcore.ErrStorageRowsNotAffected.Is(err) {
			return nil, co.errorWrapper.WrapErrorFailed(err, entity.ModelNameItem)
		}
	}

	co.eventEmitter.Emit(ctx, "RemoveItemsWithoutAttempts", mrmsg.Data{"count": len(itemsIDs)})

	return itemsIDs, nil
}

// RemoveCompletedItems - удаляет ограниченный список элементов из успешно обработанных.
// Возвращает ID элементов, которые были удалены.
func (co *QueueCleaner) RemoveCompletedItems(ctx context.Context, limit uint32) (itemsIDs []uint64, err error) {
	if co.storageBroken == nil {
		return nil, nil
	}

	if limit == 0 {
		return nil, mrcore.ErrUseCaseIncorrectInputData.New("limit", "value is zero")
	}

	itemsIDs, err = co.storageCompleted.Delete(ctx, co.completedExpiry, limit)
	if err != nil {
		if !mrcore.ErrStorageRowsNotAffected.Is(err) {
			return nil, co.errorWrapper.WrapErrorFailed(err, entity.ModelNameItem)
		}
	}

	co.eventEmitter.Emit(ctx, "RemoveCompletedItems", mrmsg.Data{"count": len(itemsIDs)})

	return itemsIDs, nil
}

// RemoveBrokenItems - удаляет ограниченный список элементов из журнала ошибок.
// Возвращает ID элементов, которые были удалены.
func (co *QueueCleaner) RemoveBrokenItems(ctx context.Context, limit uint32) (itemsIDs []uint64, err error) {
	if co.storageBroken == nil {
		return nil, nil
	}

	if limit == 0 {
		return nil, mrcore.ErrUseCaseIncorrectInputData.New("limit", "value is zero")
	}

	itemsIDs, err = co.storageBroken.Delete(ctx, co.brokenExpiry, limit)
	if err != nil {
		if !mrcore.ErrStorageRowsNotAffected.Is(err) {
			return nil, co.errorWrapper.WrapErrorFailed(err, entity.ModelNameItem)
		}
	}

	co.eventEmitter.Emit(ctx, "RemoveBrokenItems", mrmsg.Data{"count": len(itemsIDs)})

	return itemsIDs, nil
}
