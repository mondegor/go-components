package clean

import (
	"context"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrerr"

	"github.com/mondegor/go-components/mrmailer"
	"github.com/mondegor/go-components/mrqueue"
)

type (
	// MessageCleaner - объект очищающий очередь от обработанных/сломанных сообщений.
	MessageCleaner struct {
		txManager    mrstorage.DBTxManager
		storage      mrmailer.MessageStorage
		useCaseQueue mrqueue.Cleaner
		errorWrapper mrerr.UseCaseErrorWrapper
	}
)

// New - создаёт объект MessageCleaner.
func New(
	txManager mrstorage.DBTxManager,
	storage mrmailer.MessageStorage,
	useCaseQueue mrqueue.Cleaner,
	errorWrapper mrerr.UseCaseErrorWrapper,
) *MessageCleaner {
	return &MessageCleaner{
		txManager:    txManager,
		storage:      storage,
		useCaseQueue: useCaseQueue,
		errorWrapper: mrerr.NewUseCaseErrorWrapper(errorWrapper, "mrmailer.MessageCleaner"),
	}
}

// RemoveMessagesWithoutAttempts - удаляет из очереди ограниченный список сообщений находящихся
// в статусе RETRY и с нулевым кол-вом попыток в целях разгрузки очереди.
func (co *MessageCleaner) RemoveMessagesWithoutAttempts(ctx context.Context, limit int) error {
	_, err := co.useCaseQueue.RemoveItemsWithoutAttempts(ctx, limit)

	return err
}

// RemoveCompletedMessages - удаляет ограниченный список сообщений из успешно обработанных.
func (co *MessageCleaner) RemoveCompletedMessages(ctx context.Context, limit int) error {
	return co.txManager.Do(ctx, func(ctx context.Context) error {
		itemsIDs, err := co.useCaseQueue.RemoveCompletedItems(ctx, limit)
		if err != nil {
			return err
		}

		if len(itemsIDs) > 0 {
			if err = co.storage.DeleteByIDs(ctx, itemsIDs); err != nil {
				return co.errorWrapper.WrapErrorFailed(err)
			}
		}

		return nil
	})
}

// RemoveBrokenMessages - удаляет ограниченный список сообщений из журнала ошибок.
func (co *MessageCleaner) RemoveBrokenMessages(ctx context.Context, limit int) error {
	return co.txManager.Do(ctx, func(ctx context.Context) error {
		itemsIDs, err := co.useCaseQueue.RemoveBrokenItems(ctx, limit)
		if err != nil {
			return err
		}

		if len(itemsIDs) > 0 {
			if err = co.storage.DeleteByIDs(ctx, itemsIDs); err != nil {
				return co.errorWrapper.WrapErrorFailed(err)
			}
		}

		return nil
	})
}
