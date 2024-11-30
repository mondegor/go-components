package messenger

import (
	"context"

	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrlog"
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
	}
)

// New - создаёт объект Provider.
func New(messengerAPI mrsender.MessageProvider) *Provider {
	return &Provider{
		messengerAPI: messengerAPI,
	}
}

// Send - отправляет указанное сообщение.
func (p *Provider) Send(ctx context.Context, message entity.Message) error {
	if message.Data.Messenger == nil {
		return mrcore.ErrUseCaseIncorrectInputData.New("message.Data.Messenger", "nil")
	}

	mrlog.Ctx(ctx).
		Trace().
		Str("source", messengerProviderName).
		Int64("messageId", int64(message.ID)).
		Str("channel", message.Channel).
		Send()

	return p.messengerAPI.SendToChat(ctx, message.Data.Messenger.ChatID, message.Data.Messenger.Content)
}
