package handle

import (
	"context"

	"github.com/mondegor/go-sysmess/mrerr/mr"
	tracectx "github.com/mondegor/go-sysmess/mrtrace/context"

	"github.com/mondegor/go-components/mrnotifier"
	"github.com/mondegor/go-components/mrnotifier/notifier/entity"
)

type (
	// NoticeHandler - обработчик для сборки уведомлений в готовые сообщения
	// и передачи их по цепочке дальше для отправки получателю.
	NoticeHandler struct {
		useCaseBuilder mrnotifier.NoticeBuilder
		mailerAPI      mrnotifier.MailerAPI
	}
)

// New - создаёт объект NoticeHandler.
func New(useCase mrnotifier.NoticeBuilder, mailerAPI mrnotifier.MailerAPI) *NoticeHandler {
	return &NoticeHandler{
		useCaseBuilder: useCase,
		mailerAPI:      mailerAPI,
	}
}

// Execute - обрабатывает уведомления собирая их в готовые сообщения,
// и передавая их по цепочке дальше для отправки получателю.
func (co *NoticeHandler) Execute(ctx context.Context, message any) (commit func(ctx context.Context) error, err error) {
	if notice, ok := message.(entity.Notice); ok {
		ctx = co.withCorrelationIDContext(ctx, notice.Data)

		messages, err := co.useCaseBuilder.BuildNotice(ctx, notice)
		if err != nil {
			return nil, err
		}

		return func(ctx context.Context) error {
			ctx = co.withCorrelationIDContext(ctx, notice.Data)

			return co.mailerAPI.Send(ctx, messages)
		}, nil
	}

	return nil, mr.ErrInternalInvalidType.New("unknown", entity.ModelNameNotice)
}

func (co *NoticeHandler) withCorrelationIDContext(ctx context.Context, data map[string]string) context.Context {
	if correlationID, ok := data[mrnotifier.HeaderCorrelationID]; ok {
		return tracectx.WithCorrelationID(ctx, correlationID)
	}

	return ctx
}
