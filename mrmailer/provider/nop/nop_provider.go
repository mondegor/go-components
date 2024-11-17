package nop

import (
	"context"

	"github.com/mondegor/go-webcore/mrlog"

	"github.com/mondegor/go-components/mrmailer/entity"
)

const (
	nopProviderName = "NopMessageSender"
)

type (
	// Provider - заглушка реализующая интерфейс отправителя сообщений.
	Provider struct{}
)

// New - создаёт объект Provider.
func New() *Provider {
	return &Provider{}
}

// Send - эмулирует отправку сообщения.
func (p *Provider) Send(ctx context.Context, message entity.Message) error {
	mrlog.Ctx(ctx).
		Trace().
		Str("source", nopProviderName).
		Int64("messageId", int64(message.ID)).
		Str("channel", message.Channel).
		Send()

	return nil
}
