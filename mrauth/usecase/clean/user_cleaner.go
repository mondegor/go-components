package clean

import (
	"context"
	"time"

	"github.com/mondegor/go-core/errors"
)

type (
	// UserCleaner - объект, удаляющий старые записи лога активности пользователей.
	UserCleaner struct {
		storageLog   UserActivityLogStorage
		logLifeTime  time.Duration
		errorWrapper errors.Wrapper
	}

	// UserActivityLogStorage - хранилище лога активности пользователей для удаления устаревших записей.
	UserActivityLogStorage interface {
		DeleteBeforeDate(ctx context.Context, datetime time.Time, limit int) (count int, err error)
	}
)

// NewUserCleaner - создаёт объект UserCleaner.
// logLifeTime - срок хранения записей лога активности (записи старше удаляются).
func NewUserCleaner(
	storageLog UserActivityLogStorage,
	logLifeTime time.Duration,
) *UserCleaner {
	return &UserCleaner{
		storageLog:   storageLog,
		logLifeTime:  logLifeTime,
		errorWrapper: errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - удаляет одну пачку устаревших (старше logLifeTime) записей лога активности
// пользователей (не более limit). Возвращает число удалённых строк - для ItemBatchPlayer
// это сигнал "пачка была полной, есть ещё".
func (co *UserCleaner) Execute(ctx context.Context, limit int) (count int, err error) {
	if limit < 1 {
		return 0, errors.ErrInternalIncorrectInputData.WithDetails("limit is zero or negative")
	}

	count, err = co.storageLog.DeleteBeforeDate(ctx, time.Now().UTC().Add(-co.logLifeTime), limit)
	if err != nil {
		return 0, co.errorWrapper.Wrap(err)
	}

	return count, nil
}
