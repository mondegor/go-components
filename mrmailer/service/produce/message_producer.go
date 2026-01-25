package produce

import (
	"context"
	"time"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrtrace"

	"github.com/mondegor/go-components/mrmailer"
	"github.com/mondegor/go-components/mrmailer/dto"
	"github.com/mondegor/go-components/mrmailer/entity"
	"github.com/mondegor/go-components/mrqueue"
	mrqueuedto "github.com/mondegor/go-components/mrqueue/dto"
)

const (
	defaultRetryAttempts   = 3
	defaultDelayCorrection = 15 * time.Second
)

type (
	// MessageProducer - отправитель сообщений получателям.
	MessageProducer struct {
		txManager         mrstorage.DBTxManager
		sequenceGenerator mrstorage.SequenceGenerator
		storage           messageStorage
		useCaseQueue      mrqueue.Producer
		errorWrapper      errors.Wrapper
		traceManager      mrtrace.ContextManager
		retryAttempts     uint32
		delayCorrection   time.Duration
	}

	messageStorage interface {
		Insert(ctx context.Context, rows []entity.Message) error
	}
)

// New - создаёт объект MessageProducer.
func New(
	txManager mrstorage.DBTxManager,
	sequenceGenerator mrstorage.SequenceGenerator,
	storage messageStorage,
	useCaseQueue mrqueue.Producer,
	traceManager mrtrace.ContextManager,
	opts ...Option,
) *MessageProducer {
	o := options{
		sender: &MessageProducer{
			txManager:         txManager,
			sequenceGenerator: sequenceGenerator,
			storage:           storage,
			useCaseQueue:      useCaseQueue,
			errorWrapper:      errors.NewServiceWrapper(),
			traceManager:      traceManager,
			retryAttempts:     defaultRetryAttempts,
			delayCorrection:   defaultDelayCorrection,
		},
	}

	for _, opt := range opts {
		opt(&o)
	}

	return o.sender
}

// SendMessage - отправляет указанное сообщение.
func (sv *MessageProducer) SendMessage(ctx context.Context, message dto.Message) error {
	if err := sv.checkMessage(message); err != nil {
		return sv.errorWrapper.Wrap(err, "channel", message.Channel)
	}

	nextID, err := sv.sequenceGenerator.Next(ctx)
	if err != nil {
		return sv.errorWrapper.Wrap(err)
	}

	item := entity.Message{
		ID:      nextID,
		Channel: message.Channel,
		Data:    message.Data,
	}

	item.Data.Header = sv.prepareHeader(ctx, item.Data.Header)

	queueItem := mrqueuedto.Item{
		ID:            nextID,
		ReadyDelayed:  sv.getReadyDelayed(message),
		RetryAttempts: sv.getRetryAttempts(message),
	}

	return sv.txManager.Do(ctx, func(ctx context.Context) error {
		if err := sv.storage.Insert(ctx, []entity.Message{item}); err != nil {
			return sv.errorWrapper.Wrap(err)
		}

		return sv.useCaseQueue.Append(ctx, queueItem)
	})
}

// Send - отправляет указанный список сообщений.
func (sv *MessageProducer) Send(ctx context.Context, messages ...dto.Message) error {
	for i := range messages {
		if err := sv.checkMessage(messages[i]); err != nil {
			return sv.errorWrapper.Wrap(err, "channel", messages[i].Channel)
		}
	}

	countMessages := len(messages)

	nextIDs, err := sv.sequenceGenerator.MultiNext(ctx, countMessages)
	if err != nil {
		return sv.errorWrapper.Wrap(err)
	}

	itemIDs := make([]uint64, countMessages)
	items := make([]entity.Message, countMessages)
	queueItems := make([]mrqueuedto.Item, countMessages)

	for i, nextID := range nextIDs {
		itemIDs[i] = nextID

		items[i] = entity.Message{
			ID:      nextID,
			Channel: messages[i].Channel,
			Data:    messages[i].Data,
		}

		items[i].Data.Header = sv.prepareHeader(ctx, items[i].Data.Header)

		queueItems[i] = mrqueuedto.Item{
			ID:            nextID,
			ReadyDelayed:  sv.getReadyDelayed(messages[i]),
			RetryAttempts: sv.getRetryAttempts(messages[i]),
		}
	}

	return sv.txManager.Do(ctx, func(ctx context.Context) error {
		if err := sv.storage.Insert(ctx, items); err != nil {
			return sv.errorWrapper.Wrap(err)
		}

		return sv.useCaseQueue.Append(ctx, queueItems...)
	})
}

func (sv *MessageProducer) checkMessage(message dto.Message) error {
	var countBodies int

	if message.Data.Mail != nil {
		countBodies++
	}

	if message.Data.Messenger != nil {
		countBodies++
	}

	if message.Data.SMS != nil {
		countBodies++
	}

	if countBodies == 1 {
		return nil
	}

	if countBodies > 1 {
		return mrmailer.ErrInternalCheckMessageHasAFewData.New("channel", message.Channel)
	}

	return mrmailer.ErrInternalCheckMessageHasNotData.New("channel", message.Channel)
}

func (sv *MessageProducer) prepareHeader(ctx context.Context, header map[string]string) map[string]string {
	if header == nil {
		header = make(map[string]string, 4)
	}

	// // если не указан явно язык, то он выбирается из контекста
	// if v := header[mrmailer.HeaderLang]; v == "" {
	// 	header[mrmailer.HeaderLang] = mrlang.Ctx(ctx).LangCode()
	// }

	// если CorrelationID пустой, то выбирается из контекста
	if id := header[mrmailer.HeaderCorrelationID]; id == "" {
		if id = sv.traceManager.ExtractCorrelationID(ctx); id != "" {
			header[mrmailer.HeaderCorrelationID] = id
		}
	}

	return header
}

func (sv *MessageProducer) getReadyDelayed(message dto.Message) time.Duration {
	if message.SendAfter.IsZero() {
		return 0
	}

	if delayPeriod := time.Until(message.SendAfter); (delayPeriod / time.Second) > sv.delayCorrection {
		return delayPeriod
	}

	return 0
}

func (sv *MessageProducer) getRetryAttempts(message dto.Message) uint32 {
	if message.RetryAttempts > 0 {
		return message.RetryAttempts
	}

	return sv.retryAttempts
}
