package messenger

import (
	"context"

	"github.com/mondegor/go-sysmess/mrerr/mr"
	"github.com/mondegor/go-sysmess/mrtrace"
	"github.com/mondegor/go-webcore/mrsender"

	"github.com/mondegor/go-components/mrmailer/entity"
)

const (
	messengerProviderName = "MessengerSender"
)

type (
	// Provider - провайдер для отправки сообщений через заданный мессенджер.
	Provider struct {
		messengerAPI mrsender.MessageProvider
		tracer       mrtrace.Tracer
	}
)

// New - создаёт объект Provider.
func New(messengerAPI mrsender.MessageProvider, tracer mrtrace.Tracer) *Provider {
	return &Provider{
		messengerAPI: messengerAPI,
		tracer:       tracer,
	}
}

// Send - отправляет указанное сообщение.
func (p *Provider) Send(ctx context.Context, message entity.Message) error {
	if message.Data.Messenger == nil {
		return mr.ErrUseCaseIncorrectInternalInputData.New("reason", "message.Data.Messenger is nil")
	}

	p.tracer.Trace(
		ctx,
		"source", messengerProviderName,
		"messageId", message.ID,
		"channel", message.Channel,
	)

	return p.messengerAPI.SendToChat(ctx, message.Data.Messenger.ChatID, message.Data.Messenger.Content)
}
