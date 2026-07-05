package produce

import (
	"context"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/mrtrace"
	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrnotifier"
	"github.com/mondegor/go-components/mrnotifier/notifier/entity"
	"github.com/mondegor/go-components/mrqueue"
	mrqueuedto "github.com/mondegor/go-components/mrqueue/dto"
)

const (
	defaultRetryAttempts = 3
)

type (
	// NoteProducer - отправитель персонализированных уведомлений получателям.
	NoteProducer struct {
		txManager         mrstorage.DBTxManager
		sequenceGenerator mrstorage.SequenceGenerator
		storage           noteStorage
		serviceQueue      mrqueue.Producer
		errorWrapper      errors.Wrapper
		traceManager      mrtrace.ContextManager
		retryAttempts     int16
	}

	noteStorage interface {
		Insert(ctx context.Context, row entity.Note) error
	}
)

// New - создаёт объект NoteProducer.
func New(
	txManager mrstorage.DBTxManager,
	sequenceGenerator mrstorage.SequenceGenerator,
	storage noteStorage,
	serviceQueue mrqueue.Producer,
	traceManager mrtrace.ContextManager,
	opts ...Option,
) *NoteProducer {
	o := options{
		producer: &NoteProducer{
			txManager:         txManager,
			sequenceGenerator: sequenceGenerator,
			storage:           storage,
			serviceQueue:      serviceQueue,
			errorWrapper:      errors.NewServiceOperationFailedWrapper(),
			traceManager:      traceManager,
			retryAttempts:     defaultRetryAttempts,
		},
	}

	for _, opt := range opts {
		opt(&o)
	}

	return o.producer
}

// Send - отправляет уведомление, ключ которой должен быть зарегистрирован в БД компонента mrnotifier.template.
// В props можно указывать следующие служебные поля:
//   - header.lang (mrnotifier.HeaderLang) - язык уведомления (если не указан, то будет выбран автоматически);
//   - config.delayTime (mrnotifier.ConfigDelayTime) - абсолютное время (RFC3339), по истечению которого следует отправить уведомление
//     или период, на который необходимо отложить отправку уведомления (в секундах или в формате Duration);
//   - fromName (mrnotifier.FieldFromName) - адрес отправителя;
//   - to (mrnotifier.FieldTo) - адрес получателя;
//   - replyTo (mrnotifier.FieldReplyTo) - адрес для ответа на уведомление;
func (sv *NoteProducer) Send(ctx context.Context, key string, props map[string]any) error {
	if key == "" {
		return errors.ErrInternalIncorrectInputData.WithDetails("key is empty")
	}

	data := sv.prepareData(ctx, props)

	nextID, err := sv.sequenceGenerator.Next(ctx)
	if err != nil {
		return sv.errorWrapper.Wrap(err)
	}

	item := entity.Note{
		ID:   nextID,
		Key:  key,
		Data: data,
	}

	queueItem := mrqueuedto.Item{
		ID:            nextID,
		RetryAttempts: sv.retryAttempts,
	}

	err = sv.txManager.Do(ctx, func(ctx context.Context) error {
		if err = sv.storage.Insert(ctx, item); err != nil {
			return err
		}

		return sv.serviceQueue.Append(ctx, queueItem)
	})
	if err != nil {
		return sv.errorWrapper.Wrap(err)
	}

	return nil
}

func (sv *NoteProducer) prepareData(ctx context.Context, props map[string]any) map[string]string {
	data := conv.Group(props).StringMap()

	if data == nil {
		data = make(map[string]string, 4)
	}

	// если CorrelationID пустой, то выбирается из контекста
	if id := data[mrnotifier.HeaderCorrelationID]; id == "" {
		if id = sv.traceManager.ExtractCorrelationID(ctx); id != "" {
			data[mrnotifier.HeaderCorrelationID] = id
		}
	}

	// // если не указан явно язык, то он выбирается из контекста
	// if v := data[mrnotifier.HeaderLang]; v == "" {
	// 	data[mrnotifier.HeaderLang] = mrlang.Ctx(ctx).LangCode()
	// }

	return data
}
