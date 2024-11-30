package mrmailer

import (
	"time"

	"github.com/mondegor/go-webcore/mrworker/job/task"
	"github.com/mondegor/go-webcore/mrworker/process/consume"

	"github.com/mondegor/go-components/mrmailer/component/handle"
)

type (
	// ServiceOption - настройка объекта ComponentService.
	ServiceOption func(o *serviceOptions)
)

// WithChangeLimit - устанавливает опцию changeLimit для ComponentService.
func WithChangeLimit(value uint32) ServiceOption {
	return func(o *serviceOptions) {
		if o.changeLimit > 0 {
			o.changeLimit = value
		}
	}
}

// WithChangeRetryTimeout - устанавливает опцию changeRetryTimeout для ComponentService.
func WithChangeRetryTimeout(value time.Duration) ServiceOption {
	return func(o *serviceOptions) {
		if o.changeRetryTimeout > 0 {
			o.changeRetryTimeout = value
		}
	}
}

// WithChangeRetryDelayed - устанавливает опцию changeRetryDelayed для ComponentService.
func WithChangeRetryDelayed(value time.Duration) ServiceOption {
	return func(o *serviceOptions) {
		o.changeRetryDelayed = value
	}
}

// WithCleanLimit - устанавливает опцию cleanLimit для ComponentService.
func WithCleanLimit(value uint32) ServiceOption {
	return func(o *serviceOptions) {
		if o.cleanLimit > 0 {
			o.cleanLimit = value
		}
	}
}

// WithSendProcessorOpts - устанавливает опцию sendProcessor для ComponentService.
func WithSendProcessorOpts(value ...consume.Option) ServiceOption {
	return func(o *serviceOptions) {
		if len(value) > 0 {
			o.sendProcessor = append(o.sendProcessor, value...)
		}
	}
}

// WithSendHandlerOpts - устанавливает опцию sendHandler для ComponentService.
func WithSendHandlerOpts(value ...handle.Option) ServiceOption {
	return func(o *serviceOptions) {
		if len(value) > 0 {
			o.sendHandler = append(o.sendHandler, value...)
		}
	}
}

// WithTaskChangeFromToRetryOpts - устанавливает опцию taskChangeFromToRetry для ComponentService.
func WithTaskChangeFromToRetryOpts(value ...task.Option) ServiceOption {
	return func(o *serviceOptions) {
		if len(value) > 0 {
			o.taskChangeFromToRetry = append(o.taskChangeFromToRetry, value...)
		}
	}
}

// WithTaskCleanMessagesOpts - устанавливает опцию taskCleanMessages для ComponentService.
func WithTaskCleanMessagesOpts(value ...task.Option) ServiceOption {
	return func(o *serviceOptions) {
		if len(value) > 0 {
			o.taskCleanMessages = append(o.taskCleanMessages, value...)
		}
	}
}
