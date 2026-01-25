package handler

import (
	"context"

	"github.com/mondegor/go-sysmess/errors"
	tracectx "github.com/mondegor/go-sysmess/mrtrace/context"

	"github.com/mondegor/go-components/mrnotifier"
	"github.com/mondegor/go-components/mrnotifier/notifier/dto"
	"github.com/mondegor/go-components/mrnotifier/notifier/entity"
)

type (
	// SendNotice - обработчик для сборки уведомлений в готовые сообщения
	// и передачи их по цепочке дальше для отправки получателю.
	SendNotice struct {
		noticeBuilder noticeBuilder
		noticeSender  mrnotifier.NoticeSender
	}

	// noticeBuilder - собирает уведомление в форматированный вид для отправки их получателю.
	noticeBuilder interface {
		Execute(ctx context.Context, note entity.Note) (notices []dto.Notice, err error)
	}
)

// NewSendNotice - создаёт объект SendNotice.
func NewSendNotice(
	noticeBuilder noticeBuilder,
	noticeSender mrnotifier.NoticeSender,
) *SendNotice {
	return &SendNotice{
		noticeBuilder: noticeBuilder,
		noticeSender:  noticeSender,
	}
}

// Execute - обрабатывает уведомления собирая их в готовые сообщения,
// и передавая их по цепочке дальше для отправки получателю.
func (h *SendNotice) Execute(ctx context.Context, message any) (commit func(ctx context.Context) error, err error) {
	if note, ok := message.(entity.Note); ok {
		ctx = h.withCorrelationIDContext(ctx, note.Data)

		// формируется заранее, чтобы транзакция при коммите выполнилась быстрее
		notices, err := h.noticeBuilder.Execute(ctx, note)
		if err != nil {
			return nil, err
		}

		return func(ctx context.Context) error {
			ctx = h.withCorrelationIDContext(ctx, note.Data)

			return h.noticeSender.Send(ctx, notices)
		}, nil
	}

	return nil, errors.ErrInternalInvalidType.New(
		"type", "unknown",
		"expected", entity.ModelNameNotice,
	)
}

func (h *SendNotice) withCorrelationIDContext(ctx context.Context, data map[string]string) context.Context {
	if correlationID, ok := data[mrnotifier.HeaderCorrelationID]; ok {
		return tracectx.WithCorrelationID(ctx, correlationID)
	}

	return ctx
}
