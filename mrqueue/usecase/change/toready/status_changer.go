package toready

import (
	"context"
	"time"

	"github.com/mondegor/go-core/errors"
)

const (
	defaultRetryDelayed = 2 * time.Minute
)

type (
	// RetryToReadyChanger - объект изменяющий статусы сломавшихся элементов, находящихся в очереди.
	RetryToReadyChanger struct {
		storage      ItemStorage
		errorWrapper errors.Wrapper
		retryDelayed time.Duration
	}

	// ItemStorage - для перевода списка записей из статуса RETRY в статус READY.
	ItemStorage interface {
		UpdateStatusRetryToReady(ctx context.Context, delayed time.Duration, limit int) (rowIDs []uint64, err error)
	}
)

// New - создаёт объект RetryToReadyChanger.
func New(
	storage ItemStorage,
	opts ...Option,
) *RetryToReadyChanger {
	o := options{
		changer: &RetryToReadyChanger{
			storage:      storage,
			errorWrapper: errors.NewServiceRecordNotFoundWrapper(),
			retryDelayed: defaultRetryDelayed,
		},
	}

	for _, opt := range opts {
		opt(&o)
	}

	return o.changer
}

// Execute - переводит пачками элементы из статуса RETRY в статус READY
// учитывая указанную задержку нахождения элемента в этом статусе и оставшееся кол-во попыток.
func (uc *RetryToReadyChanger) Execute(ctx context.Context, limit int) (count int, err error) {
	if limit < 1 {
		return 0, errors.ErrInternalIncorrectInputData.WithDetails("limit is zero or negative")
	}

	itemsIDs, err := uc.storage.UpdateStatusRetryToReady(ctx, uc.retryDelayed, limit)
	if err != nil {
		return 0, uc.errorWrapper.Wrap(err)
	}

	return len(itemsIDs), nil
}
