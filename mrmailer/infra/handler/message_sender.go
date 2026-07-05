package handler

import (
	"context"

	"github.com/mondegor/go-core/errors"
	tracectx "github.com/mondegor/go-core/mrtrace/context"

	"github.com/mondegor/go-components/mrmailer"
	"github.com/mondegor/go-components/mrmailer/entity"
	"github.com/mondegor/go-components/mrmailer/sendmessage"
)

type (
	// SendMessage - обработчик сообщений с целью их отправки конечному получателю.
	SendMessage struct {
		senderProvider sendmessage.SenderProvider
	}
)

// NewSendMessage - создаёт объект SendMessage.
func NewSendMessage(
	senderProvider sendmessage.SenderProvider,
) *SendMessage {
	return &SendMessage{
		senderProvider: senderProvider,
	}
}

// Execute - подбирает провайдера, для конкретного сообщения
// и через него отправляет его конечному получателю.
func (h *SendMessage) Execute(_ context.Context, message entity.Message) (commit func(ctx context.Context) error, err error) {
	sender, err := h.senderProvider.Sender(message.Data)
	if err != nil {
		return nil, errors.WrapInternalError(
			err,
			"error getting client",
			"message", message.Data,
		)
	}

	return func(ctx context.Context) error {
		if id, ok := message.Data.Header[mrmailer.HeaderCorrelationID]; ok && id != "" {
			ctx = tracectx.WithCorrelationID(ctx, id)
		}

		return sender.Send(ctx, message)
	}, nil
}
