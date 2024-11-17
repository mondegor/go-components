package consume

import (
	"context"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrmsg"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrsender"
	"github.com/mondegor/go-webcore/mrsender/decorator"

	"github.com/mondegor/go-components/mrqueue"
	"github.com/mondegor/go-components/mrqueue/entity"
	"github.com/mondegor/go-components/mrqueue/enum"
)

type (
	// QueueConsumer - объект для чтения элементов из очереди и информирования о статусе их обработки.
	QueueConsumer struct {
		txManager        mrstorage.DBTxManager
		storage          mrqueue.Storage
		storageCompleted mrqueue.CompletedStorage // OPTIONAL
		storageBroken    mrqueue.BrokenStorage    // OPTIONAL
		eventEmitter     mrsender.EventEmitter
		errorWrapper     mrcore.UseCaseErrorWrapper
	}
)

// New - создаёт объект QueueConsumer.
func New(
	txManager mrstorage.DBTxManager,
	storage mrqueue.Storage,
	eventEmitter mrsender.EventEmitter,
	errorWrapper mrcore.UseCaseErrorWrapper,
	opts ...Option,
) *QueueConsumer {
	co := &QueueConsumer{
		txManager:    txManager,
		storage:      storage,
		eventEmitter: decorator.NewSourceEmitter(eventEmitter, entity.ModelNameItem),
		errorWrapper: errorWrapper,
	}

	for _, opt := range opts {
		opt(co)
	}

	return co
}

// ReadItems - читает ограниченный список элементов из очереди находящихся в статусе READY
// в порядке их добавления и переводит эти элементы в статус PROCESSING.
func (co *QueueConsumer) ReadItems(ctx context.Context, limit uint32) (itemsIDs []uint64, err error) {
	if limit == 0 {
		return nil, mrcore.ErrUseCaseIncorrectInputData.New("limit", "value is zero")
	}

	itemsIDs, err = co.storage.FetchAndUpdateStatusReadyToProcessing(ctx, limit)
	if err != nil {
		return nil, co.errorWrapper.WrapErrorFailed(err, entity.ModelNameItem)
	}

	co.eventEmitter.Emit(ctx, "ReadItems", mrmsg.Data{"count": len(itemsIDs)})

	return itemsIDs, nil
}

// CancelItems - возвращает указанные элементы в статус READY, но только
// если они находятся в статусе PROCESSING (при этом попытки не отнимаются).
func (co *QueueConsumer) CancelItems(ctx context.Context, itemsIDs []uint64) error {
	if len(itemsIDs) == 0 {
		return nil
	}

	if err := co.storage.UpdateStatusProcessingToReady(ctx, itemsIDs); err != nil {
		if !mrcore.ErrStorageRowsNotAffected.Is(err) {
			return co.errorWrapper.WrapErrorFailed(err, entity.ModelNameItem)
		}
	}

	co.eventEmitter.Emit(ctx, "CancelItems", mrmsg.Data{"count": len(itemsIDs)})

	return nil
}

// Commit - фиксирует успешный результат обработки указанного элемента очереди.
// При этом элемент удаляется из очереди и добавляется в список выполненных.
func (co *QueueConsumer) Commit(ctx context.Context, itemID uint64) error {
	if itemID == 0 {
		return mrcore.ErrUseCaseIncorrectInputData.New("item", "id is zero")
	}

	return co.txManager.Do(ctx, func(ctx context.Context) error {
		if err := co.storage.Delete(ctx, itemID, enum.ItemStatusProcessing); err != nil {
			return co.errorWrapper.WrapErrorFailed(err, entity.ModelNameItem)
		}

		if co.storageCompleted != nil {
			if err := co.storageCompleted.Insert(ctx, itemID); err != nil {
				return co.errorWrapper.WrapErrorFailed(err, entity.ModelNameItem)
			}
		}

		co.eventEmitter.Emit(ctx, "Commit", mrmsg.Data{"id": itemID})

		return nil
	})
}

// Reject - отклоняет результат обработки указанного элемента очереди с указанием причины.
// При этом элемент переводится в статус RETRY с фиксацией ошибки в журнале.
func (co *QueueConsumer) Reject(ctx context.Context, itemID uint64, causeErr error) error {
	if itemID == 0 {
		return mrcore.ErrUseCaseIncorrectInputData.New("item", "id is zero")
	}

	return co.txManager.Do(ctx, func(ctx context.Context) error {
		eventName := "Reject"

		if err := co.storage.UpdateStatusProcessingToRetry(ctx, itemID); err != nil {
			if !mrcore.ErrStorageRowsNotAffected.Is(err) {
				return co.errorWrapper.WrapErrorFailed(err, entity.ModelNameItem)
			}

			eventName += "Skipped"
		}

		if co.storageBroken != nil {
			itemWithError := entity.ItemWithError{
				ID:  itemID,
				Err: causeErr,
			}

			if err := co.storageBroken.InsertOne(ctx, itemWithError); err != nil {
				return co.errorWrapper.WrapErrorFailed(err, entity.ModelNameItem)
			}
		}

		co.eventEmitter.Emit(ctx, eventName, mrmsg.Data{"id": itemID})

		return nil
	})
}
