package sender

import (
	"context"

	"github.com/mondegor/go-components/mrmailer"
	"github.com/mondegor/go-components/mrmailer/entity"
)

type (
	// nopSender - заглушка отправителя сообщений, ничего не отправляет.
	nopSender struct{}
)

// NewNopSender - создаёт объект nopSender.
func NewNopSender() mrmailer.MessageSender {
	return nopSender{}
}

// Send - эмулирует отправку сообщения.
func (s nopSender) Send(_ context.Context, _ entity.Message) error {
	return nil
}
