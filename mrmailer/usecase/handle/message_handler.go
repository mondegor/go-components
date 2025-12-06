package handle

import (
	"context"

	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"
	"github.com/mondegor/go-sysmess/mrtrace"
	tracectx "github.com/mondegor/go-sysmess/mrtrace/context"

	"github.com/mondegor/go-components/mrmailer"
	"github.com/mondegor/go-components/mrmailer/entity"
	"github.com/mondegor/go-components/mrmailer/provider/nop"
)

type (
	// MessageHandler - обработчик сообщений с целью их отправки конечному получателю.
	MessageHandler struct {
		clientEmail     mrmailer.MessageProvider
		clientSMS       mrmailer.MessageProvider
		clientMessenger mrmailer.MessageProvider
		errorWrapper    mrerr.UseCaseErrorWrapper
	}
)

// New - создаёт объект MessageHandler.
func New(
	trace mrtrace.Tracer,
	errorWrapper mrerr.UseCaseErrorWrapper,
	opts ...Option,
) *MessageHandler {
	co := &MessageHandler{
		clientEmail:     nop.New(trace), // disabled by default
		clientSMS:       nop.New(trace), // disabled by default
		clientMessenger: nop.New(trace), // disabled by default
		errorWrapper:    mrerr.NewUseCaseErrorWrapper(errorWrapper, "mrmailer.MessageHandler"),
	}

	for _, opt := range opts {
		opt(co)
	}

	return co
}

// Execute - подбирает провайдера, для конкретного сообщения
// и через него отправляет его конечному получателю.
func (co *MessageHandler) Execute(_ context.Context, message any) (commit func(ctx context.Context) error, err error) {
	if msg, ok := message.(entity.Message); ok {
		provider, err := co.getProvider(msg)
		if err != nil {
			return nil, err
		}

		return func(ctx context.Context) error {
			if id, ok := msg.Data.Header[mrmailer.HeaderCorrelationID]; ok && id != "" {
				ctx = tracectx.WithCorrelationID(ctx, id)
			}

			return provider.Send(ctx, msg)
		}, nil
	}

	return nil, mr.ErrInternalInvalidType.New("unknown", entity.ModelNameMessage)
}

func (co *MessageHandler) getProvider(message entity.Message) (provider mrmailer.MessageProvider, err error) {
	if message.Data.Email != nil {
		if co.clientEmail == nil {
			return nil, co.errorWrapper.WrapErrorFailed(
				mrmailer.ErrProviderClientNotSpecified.New("email"),
				"channel", message.Channel,
			)
		}

		return co.clientEmail, nil
	}

	if message.Data.SMS != nil {
		if co.clientEmail == nil {
			return nil, co.errorWrapper.WrapErrorFailed(
				mrmailer.ErrProviderClientNotSpecified.New("sms"),
				"channel", message.Channel,
			)
		}

		return co.clientSMS, nil
	}

	if message.Data.Messenger != nil {
		if co.clientMessenger == nil {
			return nil, co.errorWrapper.WrapErrorFailed(
				mrmailer.ErrProviderClientNotSpecified.New("messenger"),
				"channel", message.Channel,
			)
		}

		return co.clientMessenger, nil
	}

	return nil, co.errorWrapper.WrapErrorFailed(
		mrmailer.ErrProviderClientNotSpecified.New(message.Channel),
	)
}
