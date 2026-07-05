package adapter

import (
	"context"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-webcore/mrclient"

	"github.com/mondegor/go-components/mrmailer"
	"github.com/mondegor/go-components/mrmailer/entity"
)

type (
	// messengerSender - провайдер для отправки сообщений через заданный мессенджер.
	messengerSender struct {
		clientAPI mrclient.MessengerSender
	}
)

// NewMessengerSender - создаёт объект messengerSender.
func NewMessengerSender(
	clientAPI mrclient.MessengerSender,
) mrmailer.MessageSender {
	return &messengerSender{
		clientAPI: clientAPI,
	}
}

// Send - отправляет указанное сообщение.
func (s *messengerSender) Send(ctx context.Context, message entity.Message) error {
	if message.Data.Messenger == nil {
		return errors.ErrInternalIncorrectInputData.WithDetails("message.Data.Messenger is nil")
	}

	return s.clientAPI.SendToChat(ctx, message.Data.Messenger.ChatID, message.Data.Messenger.Content)
}
