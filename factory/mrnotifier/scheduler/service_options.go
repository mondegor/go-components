package scheduler

import (
	"time"

	"github.com/mondegor/go-webcore/mrworker/job/task"
)

type (
	// ServiceOption - настройка объекта ComponentService.
	ServiceOption func(o *serviceOptions)
)

// WithCaptionPrefix - устанавливает опцию captionPrefix для ComponentService.
func WithCaptionPrefix(value string) ServiceOption {
	return func(o *serviceOptions) {
		if value != "" {
			o.captionPrefix = value
		}
	}
}

// WithChangeLimit - устанавливает опцию changeLimit для ComponentService.
func WithChangeLimit(value int) ServiceOption {
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
func WithCleanLimit(value int) ServiceOption {
	return func(o *serviceOptions) {
		if o.cleanLimit > 0 {
			o.cleanLimit = value
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

// WithTaskCleanNoticesOpts - устанавливает опцию taskCleanNotices для ComponentService.
func WithTaskCleanNoticesOpts(value ...task.Option) ServiceOption {
	return func(o *serviceOptions) {
		if len(value) > 0 {
			o.taskCleanNotices = append(o.taskCleanNotices, value...)
		}
	}
}
