package nop

import (
	"context"

	"github.com/mondegor/go-sysmess/mrtrace"

	"github.com/mondegor/go-components/mrmailer/entity"
)

const (
	nopProviderName = "NopMessageSender"
)

type (
	// Provider - заглушка реализующая интерфейс отправителя сообщений.
	Provider struct {
		tracer mrtrace.Tracer
	}
)

// New - создаёт объект Provider.
func New(tracer mrtrace.Tracer) *Provider {
	return &Provider{
		tracer: tracer,
	}
}

// Send - эмулирует отправку сообщения.
func (p *Provider) Send(ctx context.Context, message entity.Message) error {
	p.tracer.Trace(
		ctx,
		"source", nopProviderName,
		"messageId", message.ID,
		"channel", message.Channel,
	)

	return nil
}
