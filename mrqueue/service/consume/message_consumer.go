package consume

import (
	"context"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrqueue"
)

type (
	// MessageConsumer - консьюмер для обработки сообщений в очереди.
	MessageConsumer[T Message] struct {
		txManager    mrstorage.DBTxManager
		storage      messageStorage[T]
		serviceQueue mrqueue.Consumer
		errorWrapper errors.Wrapper
	}

	// Message - обрабатываемое консьюмером сообщение.
	Message interface {
		MessageID() uint64
	}

	messageStorage[T Message] interface {
		FetchByIDs(ctx context.Context, rowsIDs []uint64) ([]T, error)
	}
)

// NewMessageConsumer - создаёт объект MessageConsumer.
func NewMessageConsumer[T Message](
	txManager mrstorage.DBTxManager,
	storage messageStorage[T],
	serviceQueue mrqueue.Consumer,
) *MessageConsumer[T] {
	return &MessageConsumer[T]{
		txManager:    txManager,
		storage:      storage,
		serviceQueue: serviceQueue,
		errorWrapper: errors.NewServiceOperationFailedWrapper(),
	}
}

// ReadMessages - возвращает указанную порцию сообщений для их обработки.
func (sv *MessageConsumer[T]) ReadMessages(ctx context.Context, limit int) ([]T, error) {
	itemsIDs, err := sv.serviceQueue.ReadItems(ctx, limit)
	if err != nil {
		return nil, sv.errorWrapper.Wrap(err)
	}

	items, err := sv.storage.FetchByIDs(ctx, itemsIDs)
	if err != nil {
		return nil, sv.errorWrapper.Wrap(err)
	}

	return items, nil
}

// CancelMessages - отменяет обработку сообщений, которые были ранее считаны методом ReadMessages.
func (sv *MessageConsumer[T]) CancelMessages(ctx context.Context, messages []T) error {
	if len(messages) == 0 {
		return nil
	}

	messageIDs := make([]uint64, len(messages))

	for i := range messages {
		messageIDs[i] = messages[i].MessageID()
	}

	if err := sv.serviceQueue.CancelItems(ctx, messageIDs); err != nil {
		return sv.errorWrapper.Wrap(err)
	}

	return nil
}

// CommitMessage - закрепляет результат обработки сообщения, которое было ранее считано методом ReadMessages.
// Внешняя функция preCommit работает вместе с фиксацией результата в рамках одной транзакции.
// При работе в рамках одной БД это позволяет коммитить изменения атомарно.
func (sv *MessageConsumer[T]) CommitMessage(ctx context.Context, message T, preCommit func(ctx context.Context) error) error {
	err := sv.txManager.Do(ctx, func(ctx context.Context) error {
		if err := preCommit(ctx); err != nil {
			return err
		}

		return sv.serviceQueue.Commit(ctx, message.MessageID())
	})
	if err != nil {
		return sv.errorWrapper.Wrap(err)
	}

	return nil
}

// RejectMessage - отклоняет результат обработки сообщения, если в процессе возникла ошибка.
func (sv *MessageConsumer[T]) RejectMessage(ctx context.Context, message T, causeErr error) error {
	if err := sv.serviceQueue.Reject(ctx, message.MessageID(), causeErr); err != nil {
		return sv.errorWrapper.Wrap(err)
	}

	return nil
}

// Close - закрывает соединение консьюмера с источником данных.
func (sv *MessageConsumer[T]) Close() error {
	return nil
}
