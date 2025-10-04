package clean

import (
	"context"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"

	"github.com/mondegor/go-components/mrnotifier"
	"github.com/mondegor/go-components/mrnotifier/notifier/entity"
	"github.com/mondegor/go-components/mrqueue"
)

type (
	// NoticeCleaner - объект очищающий очередь от обработанных/сломанных уведомлений.
	NoticeCleaner struct {
		txManager    mrstorage.DBTxManager
		storage      mrnotifier.NoticeStorage
		useCaseQueue mrqueue.Cleaner
		errorWrapper mrerr.UseCaseErrorWrapper
	}
)

// New - создаёт объект NoticeCleaner.
func New(
	txManager mrstorage.DBTxManager,
	storage mrnotifier.NoticeStorage,
	useCaseQueue mrqueue.Cleaner,
	errorWrapper mrerr.UseCaseErrorWrapper,
) *NoticeCleaner {
	return &NoticeCleaner{
		txManager:    txManager,
		storage:      storage,
		useCaseQueue: useCaseQueue,
		errorWrapper: mrerr.NewUseCaseErrorWrapper(errorWrapper, entity.ModelNameNotice),
	}
}

// RemoveNoticesWithoutAttempts - удаляет из очереди ограниченный список уведомлений находящихся
// в статусе RETRY и с нулевым кол-вом попыток в целях разгрузки очереди.
func (co *NoticeCleaner) RemoveNoticesWithoutAttempts(ctx context.Context, limit int) error {
	_, err := co.useCaseQueue.RemoveItemsWithoutAttempts(ctx, limit)

	return err
}

// RemoveCompletedNotices - удаляет ограниченный список уведомлений из успешно обработанных.
func (co *NoticeCleaner) RemoveCompletedNotices(ctx context.Context, limit int) error {
	return co.txManager.Do(ctx, func(ctx context.Context) error {
		itemsIDs, err := co.useCaseQueue.RemoveCompletedItems(ctx, limit)
		if err != nil {
			return err
		}

		if len(itemsIDs) > 0 {
			if err = co.storage.DeleteByIDs(ctx, itemsIDs); err != nil {
				if !mr.ErrStorageRowsNotAffected.Is(err) {
					return co.errorWrapper.WrapErrorFailed(err)
				}
			}
		}

		return nil
	})
}

// RemoveBrokenNotices - удаляет ограниченный список уведомлений из журнала ошибок.
func (co *NoticeCleaner) RemoveBrokenNotices(ctx context.Context, limit int) error {
	return co.txManager.Do(ctx, func(ctx context.Context) error {
		itemsIDs, err := co.useCaseQueue.RemoveBrokenItems(ctx, limit)
		if err != nil {
			return err
		}

		if len(itemsIDs) > 0 {
			if err = co.storage.DeleteByIDs(ctx, itemsIDs); err != nil {
				if !mr.ErrStorageRowsNotAffected.Is(err) {
					return co.errorWrapper.WrapErrorFailed(err)
				}
			}
		}

		return nil
	})
}
