package mrqueue

import (
	"context"
	"time"

	"github.com/mondegor/go-components/mrqueue/entity"
	"github.com/mondegor/go-components/mrqueue/enum"
)

type (
	// Producer - размещает элементы в очереди для последующей их обработки.
	Producer interface {
		Append(ctx context.Context, item entity.Item) error
		Appends(ctx context.Context, items []entity.Item) error
	}

	// Consumer - читает элементы из очереди и информирует о статусе их обработки.
	Consumer interface {
		ReadItems(ctx context.Context, limit uint32) (itemsIDs []uint64, err error)
		CancelItems(ctx context.Context, itemsIDs []uint64) error
		Commit(ctx context.Context, itemID uint64) error
		Reject(ctx context.Context, itemID uint64, causeErr error) error
	}

	// Changer - изменяет статусы сломавшихся элементов, находящихся в очереди.
	Changer interface {
		ChangeProcessingToRetryByTimeout(ctx context.Context, limit uint32) (itemsIDs []uint64, err error)
		ChangeRetryToReady(ctx context.Context, limit uint32) (itemsIDs []uint64, err error)
	}

	// Cleaner - очищает очередь от обработанных/сломанных элементов.
	Cleaner interface {
		RemoveItemsWithoutAttempts(ctx context.Context, limit uint32) (itemsIDs []uint64, err error)
		RemoveCompletedItems(ctx context.Context, limit uint32) (itemsIDs []uint64, err error)
		RemoveBrokenItems(ctx context.Context, limit uint32) (itemsIDs []uint64, err error)
	}

	// Storage - предоставляет доступ к хранилищу очереди элементов.
	Storage interface {
		Insert(ctx context.Context, rows []entity.Item) error
		FetchAndUpdateStatusReadyToProcessing(ctx context.Context, limit uint32) (rowsIDs []uint64, err error)
		UpdateStatusProcessingToReady(ctx context.Context, rowsIDs []uint64) error
		UpdateStatusProcessingToRetry(ctx context.Context, rowID uint64) error
		FetchAndUpdateStatusProcessingToRetryByTimeout(ctx context.Context, timeout time.Duration, limit uint32) (rowIDs []uint64, err error)
		FetchAndUpdateStatusRetryToReady(ctx context.Context, delayed time.Duration, limit uint32) (rowIDs []uint64, err error)
		Delete(ctx context.Context, rowID uint64, status enum.ItemStatus) error
		DeleteRetryWithoutAttempts(ctx context.Context, limit uint32) (rowsIDs []uint64, err error)
	}

	// CompletedStorage - предоставляет доступ к хранилищу успешно обработанных элементов.
	CompletedStorage interface {
		Insert(ctx context.Context, rowID uint64) error
		Delete(ctx context.Context, expiry time.Duration, limit uint32) (rowsIDs []uint64, err error)
	}

	// BrokenStorage - предоставляет доступ к хранилищу сломанных элементов.
	BrokenStorage interface {
		Insert(ctx context.Context, rows []entity.ItemWithError) error
		InsertOne(ctx context.Context, row entity.ItemWithError) error
		Delete(ctx context.Context, expiry time.Duration, limit uint32) (rowsIDs []uint64, err error)
	}
)
