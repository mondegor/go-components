package handle

import (
	"context"

	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrcore/mrapp"
	"github.com/mondegor/go-webcore/mrlog"

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
		ctxFunc := func(ctx context.Context) context.Context { return ctx }

		if correlationID, ok := notice.Data[mrnotifier.HeaderCorrelationID]; ok {
			logger := mrlog.Ctx(ctx).With().Str(mrapp.KeyCorrelationID, correlationID).Logger()
			processID := mrapp.ProcessCtx(ctx) + "|" + correlationID

			ctxFunc = func(ctx context.Context) context.Context {
				return mrlog.WithContext(mrapp.WithProcessContext(ctx, processID), logger)
			}
		}

		messages, err := co.useCaseBuilder.BuildNotice(ctxFunc(ctx), notice)
		if err != nil {
			return nil, err
		}

		return func(ctx context.Context) error {
			return co.mailerAPI.Send(ctxFunc(ctx), messages)
		}, nil
	}

	return nil, mrcore.ErrInternalInvalidType.New("unknown", entity.ModelNameNotice)
}
