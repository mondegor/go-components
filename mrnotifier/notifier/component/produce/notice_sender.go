package produce

import (
	"context"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrlang"
	"github.com/mondegor/go-sysmess/mrmsg"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrcore/mrapp"

	"github.com/mondegor/go-components/mrnotifier"
	"github.com/mondegor/go-components/mrnotifier/notifier/entity"
	"github.com/mondegor/go-components/mrqueue"
	mrqueueentity "github.com/mondegor/go-components/mrqueue/entity"
)

const (
	defaultRetryAttempts = 3
)

type (
	// NoticeSender - отправитель персонализированных уведомлений получателям.
	NoticeSender struct {
		txManager         mrstorage.DBTxManager
		sequenceGenerator mrstorage.SequenceGenerator
		storage           mrnotifier.NoticeStorage
		useCaseQueue      mrqueue.Producer
		errorWrapper      mrcore.UseCaseErrorWrapper
		retryAttempts     uint32
	}
)

// New - создаёт объект NoticeSender.
func New(
	txManager mrstorage.DBTxManager,
	sequenceGenerator mrstorage.SequenceGenerator,
	storage mrnotifier.NoticeStorage,
	useCaseQueue mrqueue.Producer,
	errorWrapper mrcore.UseCaseErrorWrapper,
	opts ...Option,
) *NoticeSender {
	co := &NoticeSender{
		txManager:         txManager,
		sequenceGenerator: sequenceGenerator,
		storage:           storage,
		useCaseQueue:      useCaseQueue,
		errorWrapper:      errorWrapper,
		retryAttempts:     defaultRetryAttempts,
	}

	for _, opt := range opts {
		opt(co)
	}

	return co
}

// SendNotice - отправляет уведомление, ключ которой должен быть зарегистрирован в БД компонента mrnotifier.template.
// В props можно указывать следующие служебные поля:
//   - header.lang (mrnotifier.HeaderLang) - язык уведомления (если не указан, то будет выбран автоматически);
//   - config.delayTime (mrnotifier.ConfigDelayTime) - абсолютное время (RFC3339), по истечению которого следует отправить уведомление
//     или период, на который необходимо отложить отправку уведомления (в секундах или в формате Duration);
//   - fromName (mrnotifier.FieldFromName) - адрес отправителя;
//   - to (mrnotifier.FieldTo) - адрес получателя;
//   - replyTo (mrnotifier.FieldReplyTo) - адрес для ответа на уведомление;
func (co *NoticeSender) SendNotice(ctx context.Context, key string, props map[string]any) error {
	if key == "" {
		return mrcore.ErrUseCaseIncorrectInputData.New("notify", "key is empty")
	}

	data := co.prepareData(ctx, props)

	nextID, err := co.sequenceGenerator.Next(ctx)
	if err != nil {
		return co.errorWrapper.WrapErrorFailed(err, entity.ModelNameNotice)
	}

	item := entity.Notice{
		ID:   nextID,
		Key:  key,
		Data: data,
	}

	queueItem := mrqueueentity.Item{
		ID:            nextID,
		RetryAttempts: co.retryAttempts,
	}

	return co.txManager.Do(ctx, func(ctx context.Context) error {
		if err = co.storage.Insert(ctx, item); err != nil {
			return co.errorWrapper.WrapErrorFailed(err, entity.ModelNameNotice)
		}

		return co.useCaseQueue.Append(ctx, queueItem)
	})
}

func (co *NoticeSender) prepareData(ctx context.Context, props map[string]any) map[string]string {
	data := make(map[string]string, len(props)+2)

	for k := range props {
		data[k] = mrmsg.ToString(props[k])
	}

	// если не указан явно язык, то он выбирается из контекста
	if v := data[mrnotifier.HeaderLang]; v == "" {
		data[mrnotifier.HeaderLang] = mrlang.Ctx(ctx).LangCode()
	}

	// если CorrelationID пустой, то выбирается из контекста
	if v := data[mrnotifier.HeaderCorrelationID]; v == "" {
		data[mrnotifier.HeaderCorrelationID] = mrapp.ProcessCtx(ctx)
	}

	return data
}
