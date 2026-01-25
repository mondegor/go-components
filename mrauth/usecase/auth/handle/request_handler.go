package handle

import (
	"context"
	"encoding/json"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// RequestHandler - обработчик сообщений с целью их отправки конечному получателю.
	RequestHandler struct {
		useCase mrauth.UserStatisticUseCase
	}
)

// New - создаёт объект RequestHandler.
func New(useCase mrauth.UserStatisticUseCase, opts ...Option) *RequestHandler {
	o := options{
		handler: &RequestHandler{
			useCase: useCase,
		},
	}

	for _, opt := range opts {
		opt(&o)
	}

	return o.handler
}

// Execute - подбирает провайдера, для конкретного сообщения
// и через него отправляет его конечному получателю.
func (co *RequestHandler) Execute(ctx context.Context, messages [][]byte) error {
	list := make([]entity.UserActivityLog, len(messages))

	for i, message := range messages {
		if err := json.Unmarshal(message, &list[i]); err != nil {
			return err
		}
	}

	return co.useCase.Execute(ctx, list)
}
