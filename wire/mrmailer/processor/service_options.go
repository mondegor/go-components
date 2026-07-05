package processor

import (
	"github.com/mondegor/go-sysmess/mrprocess/consume"

	"github.com/mondegor/go-components/mrmailer/entity"
	"github.com/mondegor/go-components/mrmailer/sendmessage/provider"
)

type (
	// Option - настройка объекта consume.MessageProcessor.
	Option func(o *options)

	options struct {
		processorOpts []consume.Option[entity.Message]
		providerOpts  []provider.Option
	}
)

// WithMessageProcessorOpts - устанавливает опцию processorOpts для consume.MessageProcessor.
func WithMessageProcessorOpts(value ...consume.Option[entity.Message]) Option {
	return func(o *options) {
		o.processorOpts = append(o.processorOpts, value...)
	}
}

// WithSenderProviderOpts - устанавливает опцию providerOpts для consume.MessageProcessor.
func WithSenderProviderOpts(value ...provider.Option) Option {
	return func(o *options) {
		o.providerOpts = append(o.providerOpts, value...)
	}
}
