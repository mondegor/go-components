package consume

import (
	"context"
	"strconv"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrtype"

	"github.com/mondegor/go-components/mrmailer"
	"github.com/mondegor/go-components/mrmailer/entity"
	"github.com/mondegor/go-components/mrqueue"
)

type (
	// MessageConsumer - консьюмер для считывания сообщений
	// с целью их отправки конечному получателю.
	MessageConsumer struct {
		txManager    mrstorage.DBTxManager
		storage      mrmailer.MessageStorage
		inboxQueue   mrqueue.Consumer
		errorWrapper mrcore.UseCaseErrorWrapper
	}
)

// New - создаёт объект MessageConsumer.
func New(
	txManager mrstorage.DBTxManager,
	storage mrmailer.MessageStorage,
	useCaseQueue mrqueue.Consumer,
	errorWrapper mrcore.UseCaseErrorWrapper,
) *MessageConsumer {
	return &MessageConsumer{
		txManager:    txManager,
		storage:      storage,
		inboxQueue:   useCaseQueue,
		errorWrapper: errorWrapper,
	}
}

// ReadMessages - возвращает указанную порцию сообщений для их обработки.
func (co *MessageConsumer) ReadMessages(ctx context.Context, limit uint32) ([]any, error) {
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

// CancelMessages - отменяет обработку сообщений, которые были ранее считаны методом ReadMessages.
func (co *MessageConsumer) CancelMessages(ctx context.Context, messages []any) error {
	if len(messages) == 0 {
		return nil
	}

	itemIDs := make([]uint64, len(messages))

	for i, message := range messages {
		if item, ok := message.(entity.Message); ok {
			itemIDs[i] = item.ID

			continue
		}

		return mrcore.ErrInternalInvalidType.New("unknown", entity.ModelNameMessage+"["+strconv.Itoa(i)+"]")
	}

	return co.inboxQueue.CancelItems(ctx, itemIDs)
}

// CommitMessage - закрепляет результат обработки сообщения, которое было ранее считано методом ReadMessages.
// Внешняя функция preCommit работает вместе с фиксацией результата в рамках одной транзакции.
// При работе в рамках одной БД это позволяет коммитить изменения атомарно.
func (co *MessageConsumer) CommitMessage(ctx context.Context, message any, preCommit func(ctx context.Context) error) error {
	if item, ok := message.(entity.Message); ok {
		return co.txManager.Do(ctx, func(ctx context.Context) error {
			if err := preCommit(ctx); err != nil {
				return err
			}

			return co.inboxQueue.Commit(ctx, item.ID)
		})
	}

	return mrcore.ErrInternalInvalidType.New("unknown", entity.ModelNameMessage)
}

// RejectMessage - отклоняет результат обработки сообщения, если в процессе возникла ошибка.
func (co *MessageConsumer) RejectMessage(ctx context.Context, message any, causeErr error) error {
	if item, ok := message.(entity.Message); ok {
		return co.inboxQueue.Reject(ctx, item.ID, causeErr)
	}

	return mrcore.ErrInternalInvalidType.New("unknown", entity.ModelNameMessage)
}

// Close - закрывает соединение консьюмера с источником данных.
func (co *MessageConsumer) Close() error {
	return nil
}
