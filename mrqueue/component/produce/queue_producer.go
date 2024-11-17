package produce

import (
	"context"
	"strconv"

	"github.com/mondegor/go-sysmess/mrmsg"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrsender"
	"github.com/mondegor/go-webcore/mrsender/decorator"

	"github.com/mondegor/go-components/mrqueue"
	"github.com/mondegor/go-components/mrqueue/entity"
)

type (
	// QueueProducer - объект для размещения элементов в очереди для последующей их обработки.
	QueueProducer struct {
		storage      mrqueue.Storage
		eventEmitter mrsender.EventEmitter
		errorWrapper mrcore.UseCaseErrorWrapper
	}
)

// New - создаёт объект QueueProducer.
func New(
	storage mrqueue.Storage,
	eventEmitter mrsender.EventEmitter,
	errorWrapper mrcore.UseCaseErrorWrapper,
) *QueueProducer {
	return &QueueProducer{
		storage:      storage,
		eventEmitter: decorator.NewSourceEmitter(eventEmitter, entity.ModelNameItem),
		errorWrapper: errorWrapper,
	}
}

// Append - добавляет элемент в очередь для последующей его обработки.
func (co *QueueProducer) Append(ctx context.Context, item entity.Item) error {
	if item.ID == 0 {
		return mrcore.ErrUseCaseIncorrectInputData.New("item", "id is zero")
	}

	if item.RetryAttempts == 0 {
		return mrcore.ErrUseCaseIncorrectInputData.New("item[id="+strconv.FormatUint(item.ID, 10)+"]", "RetryAttempts is zero")
	}

	if err := co.storage.Insert(ctx, []entity.Item{item}); err != nil {
		return co.errorWrapper.WrapErrorFailed(err, entity.ModelNameItem)
	}

	co.eventEmitter.Emit(ctx, "Append", mrmsg.Data{"id": item.ID})

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
			return mrcore.ErrUseCaseIncorrectInputData.New("item["+strconv.Itoa(i)+"]", "id is zero")
		}

		if item.RetryAttempts == 0 {
			return mrcore.ErrUseCaseIncorrectInputData.New("item[id="+strconv.FormatUint(item.ID, 10)+"]", "RetryAttempts is zero")
		}

		itemsIDs[i] = item.ID
	}

	if err := co.storage.Insert(ctx, items); err != nil {
		return co.errorWrapper.WrapErrorFailed(err, entity.ModelNameItem)
	}

	co.eventEmitter.Emit(ctx, "Appends", mrmsg.Data{"ids": itemsIDs})

	return nil
}
