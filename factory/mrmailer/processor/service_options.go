package processor

import (
	"github.com/mondegor/go-webcore/mrworker/process/consume"

	"github.com/mondegor/go-components/mrmailer/usecase/handle"
)

type (
	// ServiceOption - настройка объекта ComponentService.
	ServiceOption func(o *serviceOptions)
)

// WithMessageProcessorOpts - устанавливает опцию messageProcessor для ComponentService.
func WithMessageProcessorOpts(value ...consume.Option) ServiceOption {
	return func(o *serviceOptions) {
		if len(value) > 0 {
			o.messageProcessor = append(o.messageProcessor, value...)
		}
	}
}

// WithMessageHandlerOpts - устанавливает опцию messageHandler для ComponentService.
func WithMessageHandlerOpts(value ...handle.Option) ServiceOption {
	return func(o *serviceOptions) {
		if len(value) > 0 {
			o.messageHandler = append(o.messageHandler, value...)
		}
	}
}
