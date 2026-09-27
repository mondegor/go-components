package processor

import (
	"github.com/mondegor/go-core/mrprocess/consume"

	"github.com/mondegor/go-components/mrmailer/entity"
	"github.com/mondegor/go-components/mrmailer/infra/adapter/senderrouter"
)

type (
	// Option - настройка объекта consume.MessageProcessor.
	Option func(o *options)

	options struct {
		processorOpts []consume.Option[entity.Message]
		routerOpts    []senderrouter.Option
	}
)

// WithMessageProcessorOpts - устанавливает опцию processorOpts для consume.MessageProcessor.
func WithMessageProcessorOpts(value ...consume.Option[entity.Message]) Option {
	return func(o *options) {
		o.processorOpts = append(o.processorOpts, value...)
	}
}

// WithSenderRouterOpts - устанавливает опции маршрутизатора отправителей сообщений.
func WithSenderRouterOpts(value ...senderrouter.Option) Option {
	return func(o *options) {
		o.routerOpts = append(o.routerOpts, value...)
	}
}
