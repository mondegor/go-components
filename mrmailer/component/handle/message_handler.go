package handle

import (
	"context"

	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrcore/mrapp"
	"github.com/mondegor/go-webcore/mrlog"

	"github.com/mondegor/go-components/mrmailer"
	"github.com/mondegor/go-components/mrmailer/entity"
	"github.com/mondegor/go-components/mrmailer/provider/nop"
)

type (
	// MessageHandler - обработчик сообщений с целью их отправки конечному получателю.
	MessageHandler struct {
		errorWrapper   mrcore.UseCaseErrorWrapper
		clientEmail    mrmailer.MessageProvider
		clientSMS      mrmailer.MessageProvider
		clientTelegram mrmailer.MessageProvider
	}
)

// New - создаёт объект MessageHandler.
func New(errorWrapper mrcore.UseCaseErrorWrapper, opts ...Option) *MessageHandler {
	co := &MessageHandler{
		errorWrapper:   errorWrapper,
		clientEmail:    nop.New(), // disabled by default
		clientSMS:      nop.New(), // disabled by default
		clientTelegram: nop.New(), // disabled by default
	}

	for _, opt := range opts {
		opt(co)
	}

	return co
}

// Execute - подбирает провайдера, для конкретного сообщения
// и через него отправляет его конечному получателю.
func (co *MessageHandler) Execute(ctx context.Context, message any) (commit func(ctx context.Context) error, err error) {
	if msg, ok := message.(entity.Message); ok {
		ctxFunc := func(ctx context.Context) context.Context { return ctx }

		if correlationID, ok2 := msg.Data.Header[mrmailer.HeaderCorrelationID]; ok2 {
			logger := mrlog.Ctx(ctx).With().Str(mrapp.KeyCorrelationID, correlationID).Logger()
			processID := mrapp.ProcessCtx(ctx) + "|" + correlationID

			ctxFunc = func(ctx context.Context) context.Context {
				return mrlog.WithContext(mrapp.WithProcessContext(ctx, processID), logger)
			}
		}

		provider, err := co.getProvider(msg)
		if err != nil {
			return nil, err
		}

		return func(ctx context.Context) error {
			return provider.Send(ctxFunc(ctx), msg)
		}, nil
	}

	return nil, mrcore.ErrInternalInvalidType.New("unknown", entity.ModelNameMessage)
}

func (co *MessageHandler) getProvider(message entity.Message) (provider mrmailer.MessageProvider, err error) {
	if message.Data.Email != nil {
		if co.clientEmail == nil {
			return nil, co.errorWrapper.WrapErrorEntityFailed(
				mrmailer.ErrProviderClientNotSpecified.New("email"),
				entity.ModelNameMessage,
				message.Channel,
			)
		}

		return co.clientEmail, nil
	}

	if message.Data.SMS != nil {
		if co.clientEmail == nil {
			return nil, co.errorWrapper.WrapErrorEntityFailed(
				mrmailer.ErrProviderClientNotSpecified.New("sms"),
				entity.ModelNameMessage,
				message.Channel,
			)
		}

		return co.clientSMS, nil
	}

	if message.Data.Telegram != nil {
		if co.clientTelegram == nil {
			return nil, co.errorWrapper.WrapErrorEntityFailed(
				mrmailer.ErrProviderClientNotSpecified.New("telegram"),
				entity.ModelNameMessage,
				message.Channel,
			)
		}

		return co.clientTelegram, nil
	}

	return nil, co.errorWrapper.WrapErrorFailed(
		mrmailer.ErrProviderClientNotSpecified.New(message.Channel),
		entity.ModelNameMessage,
	)
}
