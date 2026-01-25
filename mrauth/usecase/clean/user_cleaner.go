package clean

import (
	"context"
	"time"

	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth"
)

type (
	// UserCleaner - объект очищающий очередь от обработанных/сломанных уведомлений.
	UserCleaner struct {
		storageLog   mrauth.UserActivityLogStorage
		errorWrapper errors.Wrapper
	}
)

// NewUserCleaner - создаёт объект UserCleaner.
func NewUserCleaner(
	storageLog mrauth.UserActivityLogStorage,
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
