package collector

import (
	"github.com/mondegor/go-webcore/mrworker/process/collect"

	"github.com/mondegor/go-components/mrauth/usecase/auth/handle"
)

type (
	// Option - настройка объекта ComponentService.
	Option func(o *options)

	options struct {
		collectorOpts []collect.Option
		handlerOpts   []handle.Option
	}
)

// WithMessageCollectorOpts - устанавливает опцию collectorOpts для ComponentService.
func WithMessageCollectorOpts(value ...collect.Option) Option {
	return func(o *options) {
		o.collectorOpts = append(o.collectorOpts, value...)
	}
}

// WithMessageHandlerOpts - устанавливает опцию handlerOpts для ComponentService.
func WithMessageHandlerOpts(value ...handle.Option) Option {
	return func(o *options) {
		o.handlerOpts = append(o.handlerOpts, value...)
	}
}
