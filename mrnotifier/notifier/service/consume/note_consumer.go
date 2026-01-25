package consume

import (
	"context"
	"strconv"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/util/casttype"

	"github.com/mondegor/go-components/mrnotifier/notifier/entity"
	"github.com/mondegor/go-components/mrqueue"
)

type (
	// NoteConsumer - консьюмер для сборки уведомлений в готовые сообщения
	// и передачи по цепочке дальше для отправки получателю.
	NoteConsumer struct {
		txManager    mrstorage.DBTxManager
		storage      noteStorage
		serviceQueue mrqueue.Consumer
		errorWrapper errors.Wrapper
	}

	noteStorage interface {
		FetchByIDs(ctx context.Context, rowsIDs []uint64) ([]entity.Note, error)
	}
)

// New - создаёт объект NoteConsumer.
func New(
	txManager mrstorage.DBTxManager,
	storage noteStorage,
	serviceQueue mrqueue.Consumer,
) *NoteConsumer {
	return &NoteConsumer{
		txManager:    txManager,
		storage:      storage,
		serviceQueue: serviceQueue,
		errorWrapper: errors.NewServiceWrapper(),
	}
}

// ReadMessages - возвращает указанную порцию уведомлений для их обработки.
func (sv *NoteConsumer) ReadMessages(ctx context.Context, limit int) ([]any, error) {
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

// CancelMessages - отменяет обработку уведомлений, которые были ранее считаны методом ReadMessages.
func (sv *NoteConsumer) CancelMessages(ctx context.Context, messages []any) error {
	if len(messages) == 0 {
		return nil
	}

	itemIDs := make([]uint64, len(messages))

	for i, message := range messages {
		if item, ok := message.(entity.Note); ok {
			itemIDs[i] = item.ID

			continue
		}

		return errors.ErrInternalInvalidType.New(
			"type", "unknown",
			"expected", entity.ModelNameNotice+"["+strconv.Itoa(i)+"]",
		)
	}

	if err := sv.serviceQueue.CancelItems(ctx, itemIDs); err != nil {
		return sv.errorWrapper.Wrap(err)
	}

	return nil
}

// CommitMessage - закрепляет результат обработки уведомления, которое было ранее считано методом ReadMessages.
// Внешняя функция preCommit работает вместе с фиксацией результата в рамках одной транзакции.
// При работе в рамках одной БД это позволяет коммитить изменения атомарно.
func (sv *NoteConsumer) CommitMessage(ctx context.Context, message any, preCommit func(ctx context.Context) error) error {
	if item, ok := message.(entity.Note); ok {
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
		"expected", entity.ModelNameNotice,
	)
}

// RejectMessage - отклоняет результат обработки уведомления, если в процессе возникла ошибка.
func (sv *NoteConsumer) RejectMessage(ctx context.Context, message any, causeErr error) error {
	if item, ok := message.(entity.Note); ok {
		if err := sv.serviceQueue.Reject(ctx, item.ID, causeErr); err != nil {
			return sv.errorWrapper.Wrap(err)
		}

		return nil
	}

	return errors.ErrInternalInvalidType.New(
		"type", "unknown",
		"expected", entity.ModelNameNotice,
	)
}

// Close - закрывает соединение консьюмера с источником данных.
func (sv *NoteConsumer) Close() error {
	return nil
}
