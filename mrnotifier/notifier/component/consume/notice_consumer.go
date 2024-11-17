package consume

import (
	"context"
	"strconv"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrtype"

	"github.com/mondegor/go-components/mrnotifier"
	"github.com/mondegor/go-components/mrnotifier/notifier/entity"
	"github.com/mondegor/go-components/mrqueue"
)

type (
	// NoticeConsumer - консьюмер для сборки уведомлений в готовые сообщения
	// и передачи по цепочке дальше для отправки получателю.
	NoticeConsumer struct {
		txManager    mrstorage.DBTxManager
		storage      mrnotifier.NoticeStorage
		inboxQueue   mrqueue.Consumer
		errorWrapper mrcore.UseCaseErrorWrapper
	}
)

// New - создаёт объект NoticeConsumer.
func New(
	txManager mrstorage.DBTxManager,
	storage mrnotifier.NoticeStorage,
	useCaseQueue mrqueue.Consumer,
	errorWrapper mrcore.UseCaseErrorWrapper,
) *NoticeConsumer {
	return &NoticeConsumer{
		txManager:    txManager,
		storage:      storage,
		inboxQueue:   useCaseQueue,
		errorWrapper: errorWrapper,
	}
}

// ReadMessages - возвращает указанную порцию уведомлений для их обработки.
func (co *NoticeConsumer) ReadMessages(ctx context.Context, limit uint32) ([]any, error) {
	itemsIDs, err := co.inboxQueue.ReadItems(ctx, limit)
	if err != nil {
		return nil, err
	}

	items, err := co.storage.FetchByIDs(ctx, itemsIDs)
	if err != nil {
		return nil, err
	}

	return mrtype.CastSliceToAnySlice(items), nil
}

// CancelMessages - отменяет обработку уведомлений, которые были ранее считаны методом ReadMessages.
func (co *NoticeConsumer) CancelMessages(ctx context.Context, messages []any) error {
	if len(messages) == 0 {
		return nil
	}

	itemIDs := make([]uint64, len(messages))

	for i, message := range messages {
		if item, ok := message.(entity.Notice); ok {
			itemIDs[i] = item.ID

			continue
		}

		return mrcore.ErrInternalInvalidType.New("unknown", entity.ModelNameNotice+"["+strconv.Itoa(i)+"]")
	}

	return co.inboxQueue.CancelItems(ctx, itemIDs)
}

// CommitMessage - закрепляет результат обработки уведомления, которое было ранее считано методом ReadMessages.
// Внешняя функция preCommit работает вместе с фиксацией результата в рамках одной транзакции.
// При работе в рамках одной БД это позволяет коммитить изменения атомарно.
func (co *NoticeConsumer) CommitMessage(ctx context.Context, message any, preCommit func(ctx context.Context) error) error {
	if item, ok := message.(entity.Notice); ok {
		return co.txManager.Do(ctx, func(ctx context.Context) error {
			if err := preCommit(ctx); err != nil {
				return err
			}

			return co.inboxQueue.Commit(ctx, item.ID)
		})
	}

	return mrcore.ErrInternalInvalidType.New("unknown", entity.ModelNameNotice)
}

// RejectMessage - отклоняет результат обработки уведомления, если в процессе возникла ошибка.
func (co *NoticeConsumer) RejectMessage(ctx context.Context, message any, causeErr error) error {
	if item, ok := message.(entity.Notice); ok {
		return co.inboxQueue.Reject(ctx, item.ID, causeErr)
	}

	return mrcore.ErrInternalInvalidType.New("unknown", entity.ModelNameNotice)
}

// Close - закрывает соединение консьюмера с источником данных.
func (co *NoticeConsumer) Close() error {
	return nil
}
