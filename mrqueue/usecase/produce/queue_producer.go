package produce

import (
	"context"

	"github.com/mondegor/go-sysmess/mrargs"
	"github.com/mondegor/go-sysmess/mrerr/mr"
	"github.com/mondegor/go-webcore/mrsender"
	"github.com/mondegor/go-webcore/mrsender/decorator"

	core "github.com/mondegor/go-components/internal"
	"github.com/mondegor/go-components/mrqueue"
	"github.com/mondegor/go-components/mrqueue/entity"
)

type (
	// QueueProducer - объект для размещения элементов в очереди для последующей их обработки.
	QueueProducer struct {
		storage      mrqueue.Storage
		eventEmitter mrsender.EventEmitter
		errorWrapper core.UseCaseErrorWrapper
	}
)

// New - создаёт объект QueueProducer.
func New(
	storage mrqueue.Storage,
	eventEmitter mrsender.EventEmitter,
) *QueueProducer {
	return &QueueProducer{
		storage:      storage,
		eventEmitter: decorator.NewSourceEmitter(eventEmitter, entity.ModelNameItem),
		errorWrapper: core.NewUseCaseErrorWrapper(entity.ModelNameItem),
	}
}

// Append - добавляет элемент в очередь для последующей его обработки.
func (co *QueueProducer) Append(ctx context.Context, item entity.Item) error {
	if item.ID == 0 {
		return mr.ErrUseCaseIncorrectInternalInputData.New("reason", "item.SettingID is zero")
	}

	if item.RetryAttempts == 0 {
		return mr.ErrUseCaseIncorrectInternalInputData.New("reason", "item.RetryAttempts is zero", "itemId", item.ID)
	}

	if err := co.storage.Insert(ctx, []entity.Item{item}); err != nil {
		return co.errorWrapper.WrapErrorFailed(err)
	}

	co.eventEmitter.Emit(ctx, "Append", mrargs.Group{"id": item.ID})

	return nil
}

// Appends - добавляет элементы в очередь для последующей их обработки.
func (co *QueueProducer) Appends(ctx context.Context, items []entity.Item) error {
	if len(items) == 0 {
		return nil
	}

	itemsIDs := make([]uint64, len(items))

	for i, item := range items {
		if item.ID == 0 {
			return mr.ErrUseCaseIncorrectInternalInputData.New("reason", "item.SettingID is zero", "itemIndex", i)
		}

		if item.RetryAttempts == 0 {
			return mr.ErrUseCaseIncorrectInternalInputData.New("reason", "item.RetryAttempts is zero", "itemId", item.ID)
		}

		itemsIDs[i] = item.ID
	}

	if err := co.storage.Insert(ctx, items); err != nil {
		return co.errorWrapper.WrapErrorFailed(err)
	}

	co.eventEmitter.Emit(ctx, "Appends", mrargs.Group{"ids": itemsIDs})

	return nil
}
