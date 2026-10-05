package clean

import (
	"context"
	"time"

	"github.com/mondegor/go-core/errors"
)

type (
	// SecurityLogCleaner - объект, удаляющий старые записи журнала безопасности пользователей.
	SecurityLogCleaner struct {
		storageLog   SecurityLogStorage
		logLifeTime  time.Duration
		errorWrapper errors.Wrapper
	}

	// SecurityLogStorage - хранилище журнала безопасности пользователей для удаления устаревших записей.
	SecurityLogStorage interface {
		DeleteBeforeDate(ctx context.Context, datetime time.Time, limit int) (count int, err error)
	}
)

// NewSecurityLogCleaner - создаёт объект SecurityLogCleaner.
// logLifeTime - срок хранения записей журнала (записи старше удаляются).
func NewSecurityLogCleaner(
	storageLog SecurityLogStorage,
	logLifeTime time.Duration,
) *SecurityLogCleaner {
	return &SecurityLogCleaner{
		storageLog:   storageLog,
		logLifeTime:  logLifeTime,
		errorWrapper: errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - удаляет одну пачку устаревших (старше logLifeTime) записей журнала безопасности
// пользователей (не более limit). Возвращает число удалённых строк - для ItemBatchPlayer
// это сигнал "пачка была полной, есть ещё".
func (co *SecurityLogCleaner) Execute(ctx context.Context, limit int) (count int, err error) {
	if limit < 1 {
		return 0, errors.ErrInternalIncorrectInputData.WithDetails("limit is zero or negative")
	}

	count, err = co.storageLog.DeleteBeforeDate(ctx, time.Now().UTC().Add(-co.logLifeTime), limit)
	if err != nil {
		return 0, co.errorWrapper.Wrap(err)
	}

	return count, nil
}
