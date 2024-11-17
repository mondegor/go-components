package produce

import (
	"context"
	"time"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrlang"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrcore/mrapp"

	"github.com/mondegor/go-components/mrmailer"
	"github.com/mondegor/go-components/mrmailer/dto"
	"github.com/mondegor/go-components/mrmailer/entity"
	"github.com/mondegor/go-components/mrqueue"
	mrqueueentity "github.com/mondegor/go-components/mrqueue/entity"
)

const (
	defaultRetryAttempts   = 3
	defaultDelayCorrection = 15 * time.Second
)

type (
	// MessageSender - отправитель сообщений получателям.
	MessageSender struct {
		txManager         mrstorage.DBTxManager
		sequenceGenerator mrstorage.SequenceGenerator
		storage           mrmailer.MessageStorage
		useCaseQueue      mrqueue.Producer
		errorWrapper      mrcore.UseCaseErrorWrapper
		retryAttempts     uint32
		delayCorrection   time.Duration
	}
)

// New - создаёт объект MessageSender.
func New(
	txManager mrstorage.DBTxManager,
	sequenceGenerator mrstorage.SequenceGenerator,
	storage mrmailer.MessageStorage,
	useCaseQueue mrqueue.Producer,
	errorWrapper mrcore.UseCaseErrorWrapper,
	opts ...Option,
) *MessageSender {
	co := &MessageSender{
		txManager:         txManager,
		sequenceGenerator: sequenceGenerator,
		storage:           storage,
		useCaseQueue:      useCaseQueue,
		errorWrapper:      errorWrapper,
		retryAttempts:     defaultRetryAttempts,
		delayCorrection:   defaultDelayCorrection,
	}

	for _, opt := range opts {
		opt(co)
	}

	return co
}

// SendMessage - отправляет указанное сообщение.
func (co *MessageSender) SendMessage(ctx context.Context, message dto.Message) error {
	if err := co.checkMessage(message); err != nil {
		return co.errorWrapper.WrapErrorEntityFailed(err, entity.ModelNameMessage, message.Channel)
	}

	nextID, err := co.sequenceGenerator.Next(ctx)
	if err != nil {
		return co.errorWrapper.WrapErrorFailed(err, entity.ModelNameMessage)
	}

	item := entity.Message{
		ID:      nextID,
		Channel: message.Channel,
		Data:    message.Data,
	}

	item.Data.Header = co.prepareHeader(ctx, item.Data.Header)

	queueItem := mrqueueentity.Item{
		ID:            nextID,
		ReadyDelayed:  co.getReadyDelayed(message),
		RetryAttempts: co.getRetryAttempts(message),
	}

	return co.txManager.Do(ctx, func(ctx context.Context) error {
		if err := co.storage.Insert(ctx, []entity.Message{item}); err != nil {
			return co.errorWrapper.WrapErrorFailed(err, entity.ModelNameMessage)
		}

		return co.useCaseQueue.Append(ctx, queueItem)
	})
}

// Send - отправляет указанный список сообщений.
func (co *MessageSender) Send(ctx context.Context, messages []dto.Message) error {
	for i := range messages {
		if err := co.checkMessage(messages[i]); err != nil {
			return co.errorWrapper.WrapErrorEntityFailed(err, entity.ModelNameMessage, messages[i].Channel)
		}
	}

	countMessages := uint32(len(messages))

	nextIDs, err := co.sequenceGenerator.MultiNext(ctx, countMessages)
	if err != nil {
		return co.errorWrapper.WrapErrorFailed(err, entity.ModelNameMessage)
	}

	itemIDs := make([]uint64, countMessages)
	items := make([]entity.Message, countMessages)
	queueItems := make([]mrqueueentity.Item, countMessages)

	for i, nextID := range nextIDs {
		itemIDs[i] = nextID

		items[i] = entity.Message{
			ID:      nextID,
			Channel: messages[i].Channel,
			Data:    messages[i].Data,
		}

		items[i].Data.Header = co.prepareHeader(ctx, items[i].Data.Header)

		queueItems[i] = mrqueueentity.Item{
			ID:            nextID,
			ReadyDelayed:  co.getReadyDelayed(messages[i]),
			RetryAttempts: co.getRetryAttempts(messages[i]),
		}
	}

	return co.txManager.Do(ctx, func(ctx context.Context) error {
		if err := co.storage.Insert(ctx, items); err != nil {
			return co.errorWrapper.WrapErrorFailed(err, entity.ModelNameMessage)
		}

		return co.useCaseQueue.Appends(ctx, queueItems)
	})
}

func (co *MessageSender) checkMessage(message dto.Message) error {
	var countBodies uint32

	if message.Data.Email != nil {
		countBodies++
	}

	if message.Data.SMS != nil {
		countBodies++
	}

	if message.Data.Telegram != nil {
		countBodies++
	}

	if countBodies == 1 {
		return nil
	}

	if countBodies > 1 {
		return mrmailer.ErrCheckMessageHasAFewData.New(message.Channel)
	}

	return mrmailer.ErrCheckMessageHasNotData.New(message.Channel)
}

func (co *MessageSender) prepareHeader(ctx context.Context, header map[string]string) map[string]string {
	// если не указан явно язык, то он выбирается из контекста
	if v := header[mrmailer.HeaderLang]; v == "" {
		header[mrmailer.HeaderLang] = mrlang.Ctx(ctx).LangCode()
	}

	// если CorrelationID пустой, то выбирается из контекста
	if v := header[mrmailer.HeaderCorrelationID]; v == "" {
		header[mrmailer.HeaderCorrelationID] = mrapp.ProcessCtx(ctx)
	}

	return header
}

func (co *MessageSender) getReadyDelayed(message dto.Message) time.Duration {
	if message.SendAfter.IsZero() {
		return 0
	}

	if delayPeriod := time.Until(message.SendAfter); (delayPeriod / time.Second) > co.delayCorrection {
		return delayPeriod
	}

	return 0
}

func (co *MessageSender) getRetryAttempts(message dto.Message) uint32 {
	if message.RetryAttempts > 0 {
		return message.RetryAttempts
	}

	return co.retryAttempts
}
