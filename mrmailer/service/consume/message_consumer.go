package consume

import (
	"context"
	"strconv"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/util/casttype"

	"github.com/mondegor/go-components/mrmailer/entity"
	"github.com/mondegor/go-components/mrqueue"
)

type (
	// MessageConsumer - консьюмер для считывания сообщений
	// с целью их отправки конечному получателю.
	MessageConsumer struct {
		txManager    mrstorage.DBTxManager
		storage      messageStorage
		serviceQueue mrqueue.Consumer
		errorWrapper errors.Wrapper
	}

	messageStorage interface {
		FetchByIDs(ctx context.Context, rowsIDs []uint64) ([]entity.Message, error)
	}
)

// New - создаёт объект MessageConsumer.
func New(
	txManager mrstorage.DBTxManager,
	storage messageStorage,
	serviceQueue mrqueue.Consumer,
) *MessageConsumer {
	return &MessageConsumer{
		txManager:    txManager,
		storage:      storage,
		serviceQueue: serviceQueue,
		errorWrapper: errors.NewServiceWrapper(),
	}
}

// ReadMessages - возвращает указанную порцию сообщений для их обработки.
func (sv *MessageConsumer) ReadMessages(ctx context.Context, limit int) ([]any, error) {
	itemsIDs, err := sv.serviceQueue.ReadItems(ctx, limit)
	if err != nil {
		return nil, sv.errorWrapper.Wrap(err)
	}

	items, err := sv.storage.FetchByIDs(ctx, itemsIDs)
	if err != nil {
		return nil, sv.errorWrapper.Wrap(err)
	}

	return casttype.SliceToAnySlice(items), nil
}

// CancelMessages - отменяет обработку сообщений, которые были ранее считаны методом ReadMessages.
func (sv *MessageConsumer) CancelMessages(ctx context.Context, messages []any) error {
	if len(messages) == 0 {
		return nil
	}

	itemIDs := make([]uint64, len(messages))

	for i, message := range messages {
		if item, ok := message.(entity.Message); ok {
			itemIDs[i] = item.ID

			continue
		}

		return errors.ErrInternalInvalidType.New(
			"type", "unknown",
			"expected", entity.ModelNameMessage+"["+strconv.Itoa(i)+"]",
		)
	}

	if err := sv.serviceQueue.CancelItems(ctx, itemIDs); err != nil {
		return sv.errorWrapper.Wrap(err)
	}

	return nil
}

// CommitMessage - закрепляет результат обработки сообщения, которое было ранее считано методом ReadMessages.
// Внешняя функция preCommit работает вместе с фиксацией результата в рамках одной транзакции.
// При работе в рамках одной БД это позволяет коммитить изменения атомарно.
func (sv *MessageConsumer) CommitMessage(ctx context.Context, message any, preCommit func(ctx context.Context) error) error {
	if item, ok := message.(entity.Message); ok {
		err := sv.txManager.Do(ctx, func(ctx context.Context) error {
			if err := preCommit(ctx); err != nil {
				return err
			}

			return sv.serviceQueue.Commit(ctx, item.ID)
		})
		if err != nil {
			return sv.errorWrapper.Wrap(err)
		}

		return nil
	}

	return errors.ErrInternalInvalidType.New(
		"type", "unknown",
		"expected", entity.ModelNameMessage,
	)
}

// RejectMessage - отклоняет результат обработки сообщения, если в процессе возникла ошибка.
func (sv *MessageConsumer) RejectMessage(ctx context.Context, message any, causeErr error) error {
	if item, ok := message.(entity.Message); ok {
		if err := sv.serviceQueue.Reject(ctx, item.ID, causeErr); err != nil {
			return sv.errorWrapper.Wrap(err)
		}

		return nil
	}

	return errors.ErrInternalInvalidType.New(
		"type", "unknown",
		"expected", entity.ModelNameMessage,
	)
}

// Close - закрывает соединение консьюмера с источником данных.
func (sv *MessageConsumer) Close() error {
	return nil
}
