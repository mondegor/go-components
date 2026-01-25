package processor

import (
	"github.com/mondegor/go-webcore/mrworker/process/consume"
)

type (
	// Option - настройка объекта consume.MessageProcessor.
	Option func(o *options)

	options struct {
		defaultLang   string
		processorOpts []consume.Option
	}
)

// WithDefaultLang - устанавливает опцию defaultLang для consume.MessageProcessor.
func WithDefaultLang(value string) Option {
	return func(o *options) {
		o.defaultLang = value
	}
}

// WithNoticeProcessorOpts - устанавливает опцию processorOpts для consume.MessageProcessor.
func WithNoticeProcessorOpts(value ...consume.Option) Option {
	return func(o *options) {
		o.processorOpts = append(o.processorOpts, value...)
	}
}
