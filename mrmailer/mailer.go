package mrmailer

import (
	"context"

	"github.com/mondegor/go-sysmess/mrapp"
	"github.com/mondegor/go-sysmess/mrtrace"

	"github.com/mondegor/go-components/mrmailer/dto"
	"github.com/mondegor/go-components/mrmailer/entity"
)

const (
	// HeaderLang - название переменной языка сообщения.
	HeaderLang = mrapp.KeyLangCode

	// HeaderCorrelationID - название переменной заголовка, содержащего CorrelationID.
	HeaderCorrelationID = mrtrace.KeyCorrelationID
)

type (
	// MessageProducer - размещает сообщение в очереди для дальнейшей его отправки.
	MessageProducer interface {
		Send(ctx context.Context, messages ...dto.Message) error
	}

	// MessageSender - занимается непосредственной отправкой сообщения получателю.
	MessageSender interface {
		Send(ctx context.Context, message entity.Message) error
	}
)
