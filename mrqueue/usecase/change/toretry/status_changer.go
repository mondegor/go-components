package toretry

import (
	"context"
	"time"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrqueue/entity"
)

const (
	defaultRetryTimeout             = 5 * time.Minute
	causeProcessingToRetryByTimeout = "processing process has switched to retry by timeout"
)

type (
	// ProcessingToRetryChanger - объект изменяющий статусы сломавшихся элементов, находящихся в очереди.
	ProcessingToRetryChanger struct {
		txManager      mrstorage.DBTxManager
		storage        ItemStorage
		storageCrashed crashedItemStorage // OPTIONAL
		errorWrapper   errors.Wrapper
		retryTimeout   time.Duration
	}

	// ItemStorage - для перевода списка записей из статуса PROCESSING в статус RETRY, которые находятся там долгое время.
	ItemStorage interface {
		UpdateStatusProcessingToRetryByTimeout(ctx context.Context, timeout time.Duration, limit int) (rowIDs []uint64, err error)
	}

	crashedItemStorage interface {
		Insert(ctx context.Context, rows []entity.CrashedItem) error
	}
)

// New - создаёт объект ProcessingToRetryChanger.
func New(
	txManager mrstorage.DBTxManager,
	storage ItemStorage,
	opts ...Option,
) *ProcessingToRetryChanger {
	o := options{
		changer: &ProcessingToRetryChanger{
			txManager:    txManager,
			storage:      storage,
			errorWrapper: errors.NewServiceRecordNotFoundWrapper(),
			retryTimeout: defaultRetryTimeout,
		},
	}

	for _, opt := range opts {
		opt(&o)
	}

	return o.changer
}

// Execute - переводит пачками элементы из статуса PROCESSING
// в статус RETRY по таймауту (например, в случае если обработка элемента подвисла) с занесением события в журнал ошибок.
func (uc *ProcessingToRetryChanger) Execute(ctx context.Context, limit int) (count int, err error) {
	if limit < 1 {
		return 0, errors.ErrInternalIncorrectInputData.WithDetails("limit is zero or negative")
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		itemsIDs, err := uc.storage.UpdateStatusProcessingToRetryByTimeout(ctx, uc.retryTimeout, limit)
		if err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if count = len(itemsIDs); count == 0 {
			return nil
		}

		if uc.storageCrashed != nil {
			items := make([]entity.CrashedItem, count)

			for i := range itemsIDs {
				items[i].Cause = causeProcessingToRetryByTimeout
			}

			if err = uc.storageCrashed.Insert(ctx, items); err != nil {
				return uc.errorWrapper.Wrap(err)
			}
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	return count, nil
}
