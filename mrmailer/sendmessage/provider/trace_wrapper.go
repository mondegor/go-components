package provider

import (
	"context"

	"github.com/mondegor/go-core/mrtrace"

	"github.com/mondegor/go-components/mrmailer"
	"github.com/mondegor/go-components/mrmailer/entity"
)

type (
	traceWrapper struct {
		tracer mrtrace.Tracer
		source string
		sender mrmailer.MessageSender
	}
)

func newTraceWrapper(
	tracer mrtrace.Tracer,
	source string,
	sender mrmailer.MessageSender,
) mrmailer.MessageSender {
	return &traceWrapper{
		tracer: tracer,
		source: source,
		sender: sender,
	}
}

// Send - эмулирует отправку сообщения.
func (p *traceWrapper) Send(ctx context.Context, message entity.Message) error {
	p.tracer.Trace(
		ctx,
		"source", p.source,
		"messageId", message.ID,
		"channel", message.Channel,
	)

	return p.sender.Send(ctx, message)
}
