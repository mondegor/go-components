package mrnotifier

import (
	"time"

	"github.com/mondegor/go-webcore/mrworker/job/task"
	"github.com/mondegor/go-webcore/mrworker/process/consume"
)

type (
	// ServiceOption - настройка объекта NoticeService.
	ServiceOption func(o *serviceOptions)
)

// WithChangeLimit - устанавливает опцию changeLimit для Service.
func WithChangeLimit(value uint32) ServiceOption {
	return func(o *serviceOptions) {
		if o.changeLimit > 0 {
			o.changeLimit = value
		}
	}
}

// WithChangeRetryTimeout - устанавливает опцию changeRetryTimeout для Service.
func WithChangeRetryTimeout(value time.Duration) ServiceOption {
	return func(o *serviceOptions) {
		if o.changeRetryTimeout > 0 {
			o.changeRetryTimeout = value
		}
	}
}

// WithChangeRetryDelayed - устанавливает опцию changeRetryDelayed для Service.
func WithChangeRetryDelayed(value time.Duration) ServiceOption {
	return func(o *serviceOptions) {
		o.changeRetryDelayed = value
	}
}

// WithCleanLimit - устанавливает опцию cleanLimit для Service.
func WithCleanLimit(value uint32) ServiceOption {
	return func(o *serviceOptions) {
		if o.cleanLimit > 0 {
			o.cleanLimit = value
		}
	}
}

// WithDefaultLang - устанавливает опцию defaultLang для Service.
func WithDefaultLang(value string) ServiceOption {
	return func(o *serviceOptions) {
		if o.defaultLang != "" {
			o.defaultLang = value
		}
	}
}

// WithSendProcessorOpts - устанавливает опцию sendProcessor для Service.
func WithSendProcessorOpts(value ...consume.Option) ServiceOption {
	return func(o *serviceOptions) {
		if len(value) > 0 {
			o.sendProcessor = append(o.sendProcessor, value...)
		}
	}
}

// WithTaskChangeFromToRetryOpts - устанавливает опцию taskChangeFromToRetry для Service.
func WithTaskChangeFromToRetryOpts(value ...task.Option) ServiceOption {
	return func(o *serviceOptions) {
		if len(value) > 0 {
			o.taskChangeFromToRetry = append(o.taskChangeFromToRetry, value...)
		}
	}
}

// WithTaskCleanNoticesOpts - устанавливает опцию taskCleanNotices для Service.
func WithTaskCleanNoticesOpts(value ...task.Option) ServiceOption {
	return func(o *serviceOptions) {
		if len(value) > 0 {
			o.taskCleanNotices = append(o.taskCleanNotices, value...)
		}
	}
}
