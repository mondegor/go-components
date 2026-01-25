package sendmessage

import (
	"github.com/mondegor/go-components/mrmailer"
	"github.com/mondegor/go-components/mrmailer/entity"
)

type (
	// SenderProvider - интерфейс получения провайдера отправки сообщений.
	SenderProvider interface {
		Sender(data entity.MessageData) (mrmailer.MessageSender, error)
	}
)
