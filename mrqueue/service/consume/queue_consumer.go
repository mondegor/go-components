package consume

import (
	"context"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/errors/kind"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrqueue/entity"
	"github.com/mondegor/go-components/mrqueue/enum/itemstatus"
)

type (
	// QueueConsumer - объект для чтения элементов из очереди и информирования о статусе их обработки.
	QueueConsumer struct {
		txManager        mrstorage.DBTxManager
		storage          itemStorage
		storageCompleted completedItemStorage // OPTIONAL
		storageCrashed   crashedItemStorage   // OPTIONAL
		errorWrapper     errors.Wrapper
	}

	itemStorage interface {
		FetchAndUpdateStatusReadyToProcessing(ctx context.Context, limit int) (rowsIDs []uint64, err error)
		UpdateStatusProcessingToReady(ctx context.Context, rowsIDs []uint64) error
		UpdateStatusProcessingToRetry(ctx context.Context, rowID uint64) error
		Delete(ctx context.Context, rowID uint64, status itemstatus.Enum) error
	}

	completedItemStorage interface {
		Insert(ctx context.Context, rowID uint64) error
	}

	crashedItemStorage interface {
		InsertOne(ctx context.Context, row entity.CrashedItem) error
	}
)

var errSystemNoProcessingRowFound = errors.NewSystemProto("no processing row found")

// NewQueueConsumer - создаёт объект QueueConsumer.
func NewQueueConsumer(
	txManager mrstorage.DBTxManager,
	storage itemStorage,
	opts ...Option,
) *QueueConsumer {
	o := options{
		consumer: &QueueConsumer{
			txManager:    txManager,
			storage:      storage,
			errorWrapper: errors.NewServiceOperationFailedWrapper(),
		},
	}

	for _, opt := range opts {
		opt(&o)
	}

	return o.consumer
}

// ReadItems - читает ограниченный список элементов из очереди находящихся в статусе READY
// в порядке их добавления и переводит эти элементы в статус PROCESSING.
func (sv *QueueConsumer) ReadItems(ctx context.Context, limit int) (itemsIDs []uint64, err error) {
	if limit < 1 {
		return nil, errors.ErrInternalIncorrectInputData.WithDetails("limit is zero or negative")
	}

	itemsIDs, err = sv.storage.FetchAndUpdateStatusReadyToProcessing(ctx, limit)
	if err != nil {
		return nil, sv.errorWrapper.Wrap(err)
	}

	return itemsIDs, nil
}

// CancelItems - возвращает указанные элементы в статус READY, но только
// если они находятся в статусе PROCESSING (при этом попытки не отнимаются).
func (sv *QueueConsumer) CancelItems(ctx context.Context, itemsIDs []uint64) error {
	if len(itemsIDs) == 0 {
		return nil
	}

	if err := sv.storage.UpdateStatusProcessingToReady(ctx, itemsIDs); err != nil {
		if errors.Is(err, errors.ErrEventStorageRecordsNotAffected) {
			return nil
		}

		return sv.errorWrapper.Wrap(err)
	}

	return nil
}

// Commit - фиксирует успешный результат обработки указанного элемента очереди.
// При этом элемент удаляется из очереди и добавляется в список выполненных.
func (sv *QueueConsumer) Commit(ctx context.Context, itemID uint64) error {
	if itemID == 0 {
		return errors.ErrInternalIncorrectInputData.WithDetails("itemID is zero")
	}

	return sv.txManager.Do(ctx, func(ctx context.Context) error {
		if err := sv.storage.Delete(ctx, itemID, itemstatus.Processing); err != nil {
			return sv.errorWrapper.Wrap(err)
		}

		if sv.storageCompleted != nil {
			if err := sv.storageCompleted.Insert(ctx, itemID); err != nil {
				return sv.errorWrapper.Wrap(err)
			}
		}

		return nil
	})
}

// Reject - отклоняет результат обработки указанного элемента очереди с указанием причины ошибки.
// Если причина ошибки типа System, то элемент переводится в статус RETRY с фиксацией ошибки в журнале.
// Иначе элемент удаляется из очереди с фиксацией уточнённой ошибки в журнале.
func (sv *QueueConsumer) Reject(ctx context.Context, itemID uint64, causeErr error) error {
	if itemID == 0 {
		return errors.ErrInternalIncorrectInputData.WithDetails("itemID is zero")
	}

	return sv.txManager.Do(ctx, func(ctx context.Context) error {
		switch kind.Extract(causeErr) {
		case kind.System:
			if err := sv.storage.UpdateStatusProcessingToRetry(ctx, itemID); err != nil {
				if !errors.Is(err, errors.ErrEventStorageNoRecordFound) {
					return sv.errorWrapper.Wrap(err)
				}

				causeErr = errSystemNoProcessingRowFound.Wrap(causeErr)
			}
		default:
			if err := sv.storage.Delete(ctx, itemID, itemstatus.Processing); err != nil {
				return sv.errorWrapper.Wrap(err)
			}
		}

		if sv.storageCrashed != nil {
			crashedItem := entity.CrashedItem{
				ID:    itemID,
				Cause: causeErr.Error(),
			}

			if err := sv.storageCrashed.InsertOne(ctx, crashedItem); err != nil {
				return sv.errorWrapper.Wrap(err)
			}
		}

		return nil
	})
}
