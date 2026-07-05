package mrqueue

import (
	"context"

	"github.com/mondegor/go-components/mrqueue/dto"
)

type (
	// Producer - размещает элементы в очереди для последующей их обработки.
	Producer interface {
		Append(ctx context.Context, item ...dto.Item) error
	}

	// Consumer - читает элементы из очереди и информирует о статусе их обработки.
	Consumer interface {
		ReadItems(ctx context.Context, limit int) (itemsIDs []uint64, err error)
		CancelItems(ctx context.Context, itemsIDs []uint64) error
		Commit(ctx context.Context, itemID uint64) error
		Reject(ctx context.Context, itemID uint64, causeErr error) error
	}
)
