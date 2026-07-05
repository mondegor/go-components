package clean

import (
	"context"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
)

type (
	// QueueCleaner - объект очищающий очередь от обработанных/сломанных элементов.
	QueueCleaner struct {
		txManager      mrstorage.DBTxManager
		storage        ItemStorage
		afterCleanFunc func(ctx context.Context, itemsIDs []uint64) error
		errorWrapper   errors.Wrapper
	}

	// ItemStorage - для удаления из очереди списка записей находящихся в статусе RETRY.
	ItemStorage interface {
		DeleteRetryWithoutAttempts(ctx context.Context, limit int) (rowsIDs []uint64, err error)
	}
)

// New - создаёт объект QueueCleaner.
func New(
	txManager mrstorage.DBTxManager,
	storage ItemStorage,
	opts ...Option,
) *QueueCleaner {
	o := options{
		cleaner: &QueueCleaner{
			txManager:    txManager,
			storage:      storage,
			errorWrapper: errors.NewServiceRecordNotFoundWrapper(),
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

// Execute - удаляет из очереди пачками элементы находящихся
// в статусе RETRY и с нулевым кол-вом попыток в целях разгрузки очереди.
// Возвращает ID элементов, которые были удалены.
func (uc *QueueCleaner) Execute(ctx context.Context, limit int) (count int, err error) {
	if limit < 1 {
		return 0, errors.ErrInternalIncorrectInputData.WithDetails("limit is zero or negative")
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		itemsIDs, err := uc.storage.DeleteRetryWithoutAttempts(ctx, limit)
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
