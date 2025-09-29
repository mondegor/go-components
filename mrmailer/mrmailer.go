package mrmailer

import (
	"context"

	"github.com/mondegor/go-sysmess/mrtrace"

	core "github.com/mondegor/go-components/internal"
	"github.com/mondegor/go-components/mrmailer/dto"
	"github.com/mondegor/go-components/mrmailer/entity"
)

const (
	// HeaderLang - название переменной языка сообщения.
	HeaderLang = core.KeyLangCode

	// HeaderCorrelationID - название переменной заголовка, содержащего CorrelationID.
	HeaderCorrelationID = mrtrace.KeyCorrelationID
)

type (
	// MessageProducer - размещает сообщение в очереди для дальнейшей отправки.
	MessageProducer interface {
		SendMessage(ctx context.Context, message dto.Message) error
		Send(ctx context.Context, messages []dto.Message) error
	}

	// MessageProvider - провайдер, который занимается непосредственной отправкой сообщения получателю.
	MessageProvider interface {
		Send(ctx context.Context, message entity.Message) error
	}

	// MessageStorage - предоставляет доступ к хранилищу сообщений.
	MessageStorage interface {
		FetchByIDs(ctx context.Context, rowsIDs []uint64) ([]entity.Message, error)
		Insert(ctx context.Context, rows []entity.Message) error
		DeleteByIDs(ctx context.Context, rowsIDs []uint64) error
	}
)
