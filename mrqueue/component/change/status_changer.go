package change

import (
	"context"
	"errors"
	"time"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrmsg"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrsender"
	"github.com/mondegor/go-webcore/mrsender/decorator"

	"github.com/mondegor/go-components/mrqueue"
	"github.com/mondegor/go-components/mrqueue/entity"
)

const (
	defaultRetryTimeout = 5 * time.Minute
	defaultRetryDelayed = 2 * time.Minute
)

type (
	// StatusChanger - объект изменяющий статусы сломавшихся элементов, находящихся в очереди.
	StatusChanger struct {
		txManager     mrstorage.DBTxManager
		storage       mrqueue.Storage
		storageBroken mrqueue.BrokenStorage // OPTIONAL
		eventEmitter  mrsender.EventEmitter
		errorWrapper  mrcore.UseCaseErrorWrapper
		retryTimeout  time.Duration
		retryDelayed  time.Duration
	}
)

var errProcessingToRetryByTimeout = errors.New("processing process has switched to retry by timeout")

// New - создаёт объект StatusChanger.
func New(
	txManager mrstorage.DBTxManager,
	storage mrqueue.Storage,
	eventEmitter mrsender.EventEmitter,
	errorWrapper mrcore.UseCaseErrorWrapper,
	opts ...Option,
) *StatusChanger {
	co := &StatusChanger{
		txManager:    txManager,
		storage:      storage,
		eventEmitter: decorator.NewSourceEmitter(eventEmitter, entity.ModelNameItem),
		errorWrapper: errorWrapper,
		retryTimeout: defaultRetryTimeout,
		retryDelayed: defaultRetryDelayed,
	}

	for _, opt := range opts {
		opt(co)
	}

	return co
}

// ChangeProcessingToRetryByTimeout - переводит ограниченный список элементов из статуса PROCESSING
// в статус RETRY по таймауту (например, в случае если обработка элемента подвисла) с занесением события в журнал ошибок.
func (co *StatusChanger) ChangeProcessingToRetryByTimeout(ctx context.Context, limit uint32) (itemsIDs []uint64, err error) {
	if limit == 0 {
		return nil, mrcore.ErrUseCaseIncorrectInputData.New("limit", "value is zero")
	}

	err = co.txManager.Do(ctx, func(ctx context.Context) error {
		itemsIDs, err = co.storage.FetchAndUpdateStatusProcessingToRetryByTimeout(ctx, co.retryTimeout, limit)
		if err != nil {
			return co.errorWrapper.WrapErrorFailed(err, entity.ModelNameItem)
		}

		if co.storageBroken != nil {
			items := make([]entity.ItemWithError, len(itemsIDs))

			for i := range itemsIDs {
				items[i].Err = errProcessingToRetryByTimeout
			}

			if err = co.storageBroken.Insert(ctx, items); err != nil {
				return co.errorWrapper.WrapErrorFailed(err, entity.ModelNameItem)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	co.eventEmitter.Emit(ctx, "ChangeProcessingToRetryByTimeout", mrmsg.Data{"count": len(itemsIDs)})

	return itemsIDs, err
}

// ChangeRetryToReady - переводит ограниченный список элементов из статуса RETRY в статус READY
// учитывая указанную задержку нахождения элемента в этом статусе и положительное кол-во попыток.
func (co *StatusChanger) ChangeRetryToReady(ctx context.Context, limit uint32) (itemsIDs []uint64, err error) {
	if limit == 0 {
		return nil, mrcore.ErrUseCaseIncorrectInputData.New("limit", "value is zero")
	}

	itemsIDs, err = co.storage.FetchAndUpdateStatusRetryToReady(ctx, co.retryDelayed, limit)
	if err != nil {
		return nil, co.errorWrapper.WrapErrorFailed(err, entity.ModelNameItem)
	}

	co.eventEmitter.Emit(ctx, "ChangeRetryToReady", mrmsg.Data{"count": len(itemsIDs)})

	return itemsIDs, nil
}
