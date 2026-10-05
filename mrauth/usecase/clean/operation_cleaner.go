package clean

import (
	"context"

	"github.com/mondegor/go-core/errors"
)

type (
	// OperationCleaner - объект, удаляющий просроченные защищённые операции.
	OperationCleaner struct {
		storage      OperationStorage
		errorWrapper errors.Wrapper
	}

	// OperationStorage - хранилище защищённых операций для удаления просроченных.
	OperationStorage interface {
		DeleteExpired(ctx context.Context, limit int) (count int, err error)
	}
)

// NewOperationCleaner - создаёт объект OperationCleaner.
func NewOperationCleaner(storage OperationStorage) *OperationCleaner {
	return &OperationCleaner{
		storage:      storage,
		errorWrapper: errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - удаляет одну пачку просроченных защищённых операций (не более limit).
// Возвращает число удалённых строк - для ItemBatchPlayer это сигнал "пачка была полной, есть ещё".
func (co *OperationCleaner) Execute(ctx context.Context, limit int) (count int, err error) {
	if limit < 1 {
		return 0, errors.ErrInternalIncorrectInputData.WithDetails("limit is zero or negative")
	}

	count, err = co.storage.DeleteExpired(ctx, limit)
	if err != nil {
		return 0, co.errorWrapper.Wrap(err)
	}

	return count, nil
}
