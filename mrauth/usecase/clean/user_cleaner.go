package clean

import (
	"context"
	"time"

	"github.com/mondegor/go-sysmess/errors"
)

type (
	// UserCleaner - объект очищающий очередь от обработанных/сломанных уведомлений.
	UserCleaner struct {
		storageLog   userActivityLogStorage
		errorWrapper errors.Wrapper
	}

	userActivityLogStorage interface {
		DeleteBeforeDate(ctx context.Context, datetime time.Time, limit int) error
	}
)

// NewUserCleaner - создаёт объект UserCleaner.
func NewUserCleaner(
	storageLog userActivityLogStorage,
) *UserCleaner {
	return &UserCleaner{
		storageLog:   storageLog,
		errorWrapper: errors.NewUseCaseWrapper(),
	}
}

// RemoveOldLog - удаляет ограниченный список уведомлений из успешно обработанных.
func (co *UserCleaner) RemoveOldLog(ctx context.Context, logLifeTime time.Duration, limit int) error {
	if err := co.storageLog.DeleteBeforeDate(ctx, time.Now().Add(-logLifeTime), limit); err != nil {
		return co.errorWrapper.Wrap(err)
	}

	return nil
}
