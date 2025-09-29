package clean

import (
	"context"
	"time"

	core "github.com/mondegor/go-components/internal"
	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// UserCleaner - объект очищающий очередь от обработанных/сломанных уведомлений.
	UserCleaner struct {
		storageLog   mrauth.UserActivityLogStorage
		errorWrapper core.UseCaseErrorWrapper
	}
)

// NewUserCleaner - создаёт объект UserCleaner.
func NewUserCleaner(storageLog mrauth.UserActivityLogStorage) *UserCleaner {
	return &UserCleaner{
		storageLog:   storageLog,
		errorWrapper: core.NewUseCaseErrorWrapper(entity.ModelNameUser),
	}
}

// RemoveOldLog - удаляет ограниченный список уведомлений из успешно обработанных.
func (co *UserCleaner) RemoveOldLog(ctx context.Context, logLifeTime time.Duration, limit int) error {
	if err := co.storageLog.DeleteBeforeDate(ctx, time.Now().Add(-logLifeTime), limit); err != nil {
		return co.errorWrapper.WrapErrorFailed(err)
	}

	return nil
}
