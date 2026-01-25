package produce

import (
	"context"

	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrqueue/dto"
)

type (
	// QueueProducer - объект для размещения элементов в очереди для последующей их обработки.
	QueueProducer struct {
		storage      itemStorage
		errorWrapper errors.Wrapper
	}

	itemStorage interface {
		Insert(ctx context.Context, rows []dto.Item) error
	}
)

// New - создаёт объект QueueProducer.
func New(
	storage itemStorage,
) *QueueProducer {
	return &QueueProducer{
		storage:      storage,
		errorWrapper: errors.NewServiceWrapper(),
	}
}

// Append - добавляет элементы в очередь для последующей их обработки.
func (sv *QueueProducer) Append(ctx context.Context, items ...dto.Item) error {
	if len(items) == 0 {
		return nil
	}

	for i, item := range items {
		if item.ID == 0 {
			return errors.ErrInternalIncorrectInputData.WithDetails(
				"item.ID is zero",
				"itemIndex", i,
			)
		}

		if item.RetryAttempts == 0 {
			return errors.ErrInternalIncorrectInputData.WithDetails(
				"item.RetryAttempts is zero",
				"itemId", item.ID,
			)
		}
	}

	if err := sv.storage.Insert(ctx, items); err != nil {
		return sv.errorWrapper.Wrap(err)
	}

	return nil
}
