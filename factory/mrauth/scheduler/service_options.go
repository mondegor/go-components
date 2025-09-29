package scheduler

import (
	"time"

	"github.com/mondegor/go-webcore/mrworker/job/task"
)

type (
	// ServiceOption - настройка объекта ComponentService.
	ServiceOption func(o *serviceOptions)
)

// WithCaptionPrefix - устанавливает опцию caption для ComponentService.
func WithCaptionPrefix(value string) ServiceOption {
	return func(o *serviceOptions) {
		if value != "" {
			o.captionPrefix = value
		}
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

// WithLogLifeTime - устанавливает опцию cleanLimit для ComponentService.
func WithLogLifeTime(value time.Duration) ServiceOption {
	return func(o *serviceOptions) {
		if o.logLifeTime > 0 {
			o.logLifeTime = value
		}
	}
}

// WithTaskCleanRecordsOpts - устанавливает опцию taskCleanMessages для ComponentService.
func WithTaskCleanRecordsOpts(value ...task.Option) ServiceOption {
	return func(o *serviceOptions) {
		if len(value) > 0 {
			o.taskCleanRecords = append(o.taskCleanRecords, value...)
		}
	}
}
