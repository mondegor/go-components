package clean

import (
	"context"
	"time"

	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"
)

const (
	defaultCrashedExpiry = 72 * time.Hour
)

type (
	// CrashedItemsCleaner - объект очищающий очередь от обработанных/сломанных элементов.
	CrashedItemsCleaner struct {
		txManager      mrstorage.DBTxManager
		storage        ItemStorage
		afterCleanFunc func(ctx context.Context, itemsIDs []uint64) error
		errorWrapper   errors.Wrapper
		crashedExpiry  time.Duration
	}

	// ItemStorage - для удаления списка записей из журнала ошибок.
	ItemStorage interface {
		Delete(ctx context.Context, expiry time.Duration, limit int) (rowsIDs []uint64, err error)
	}
)

// New - создаёт объект CrashedItemsCleaner.
func New(
	txManager mrstorage.DBTxManager,
	storage ItemStorage,
	opts ...Option,
) *CrashedItemsCleaner {
	o := options{
		cleaner: &CrashedItemsCleaner{
			txManager:     txManager,
			storage:       storage,
			errorWrapper:  errors.NewServiceRecordNotFoundWrapper(),
			crashedExpiry: defaultCrashedExpiry,
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

// Execute - удаляет пачками элементы из журнала ошибок.
// Возвращает ID элементов, которые были удалены.
func (uc *CrashedItemsCleaner) Execute(ctx context.Context, limit int) (count int, err error) {
	if limit < 1 {
		return 0, errors.ErrInternalIncorrectInputData.WithDetails("limit is zero or negative")
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		itemsIDs, err := uc.storage.Delete(ctx, uc.crashedExpiry, limit)
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
