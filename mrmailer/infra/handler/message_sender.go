package handler

import (
	"context"

	"github.com/mondegor/go-sysmess/errors"
	tracectx "github.com/mondegor/go-sysmess/mrtrace/context"

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
func (h *SendMessage) Execute(_ context.Context, message any) (commit func(ctx context.Context) error, err error) {
	if msg, ok := message.(entity.Message); ok {
		sender, err := h.senderProvider.Sender(msg.Data)
		if err != nil {
			return nil, errors.WrapInternalError(
				err,
				"error getting client",
				"message", msg.Data,
			)
		}

		return func(ctx context.Context) error {
			if id, ok := msg.Data.Header[mrmailer.HeaderCorrelationID]; ok && id != "" {
				ctx = tracectx.WithCorrelationID(ctx, id)
			}

			return sender.Send(ctx, msg)
		}, nil
	}

	return nil, errors.ErrInternalInvalidType.New(
		"type", "unknown",
		"expected", entity.ModelNameMessage,
	)
}
