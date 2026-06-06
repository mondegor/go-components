package clean

import (
	"context"
	"time"

	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"
)

const (
	defaultCompletedExpiry = 24 * time.Hour
)

type (
	// CompletedItemsCleaner - объект очищающий очередь от обработанных/сломанных элементов.
	CompletedItemsCleaner struct {
		txManager       mrstorage.DBTxManager
		storage         ItemStorage
		afterCleanFunc  func(ctx context.Context, itemsIDs []uint64) error
		errorWrapper    errors.Wrapper
		completedExpiry time.Duration
	}

	// ItemStorage - для удаления списка записей из успешно обработанных.
	ItemStorage interface {
		Delete(ctx context.Context, expiry time.Duration, limit int) (rowsIDs []uint64, err error)
	}
)

// New - создаёт объект CompletedItemsCleaner.
func New(
	txManager mrstorage.DBTxManager,
	storage ItemStorage,
	opts ...Option,
) *CompletedItemsCleaner {
	o := options{
		cleaner: &CompletedItemsCleaner{
			txManager:       txManager,
			storage:         storage,
			errorWrapper:    errors.NewServiceRecordNotFoundWrapper(),
			completedExpiry: defaultCompletedExpiry,
		},
	}

	for _, opt := range opts {
		opt(&o)
	}

	if o.cleaner.afterCleanFunc == nil {
		o.cleaner.afterCleanFunc = func(_ context.Context, _ []uint64) error {
			return nil
		}
	}

	return o.cleaner
}

// Execute - удаляет пачками элементы из успешно обработанных.
// Возвращает ID элементов, которые были удалены.
func (uc *CompletedItemsCleaner) Execute(ctx context.Context, limit int) (count int, err error) {
	if limit < 1 {
		return 0, errors.ErrInternalIncorrectInputData.WithDetails("limit is zero or negative")
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		itemsIDs, err := uc.storage.Delete(ctx, uc.completedExpiry, limit)
		if err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if count = len(itemsIDs); count == 0 {
			return nil
		}

		if err = uc.afterCleanFunc(ctx, itemsIDs); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	return count, nil
}
