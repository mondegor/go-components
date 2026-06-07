package scheduler

import (
	"time"

	"github.com/mondegor/go-sysmess/mrprocess/job/task"
)

type (
	// Option - настройка объекта ComponentService.
	Option func(o *options)

	options struct {
		captionPrefix   string
		cleanLimit      int
		logLifeTime     time.Duration
		taskCleanerOpts []task.Option
	}
)

// WithCaptionPrefix - устанавливает опцию caption для ComponentService.
func WithCaptionPrefix(value string) Option {
	return func(o *options) {
		o.captionPrefix = value
	}
}

// WithCleanLimit - устанавливает опцию cleanLimit для ComponentService.
func WithCleanLimit(value int) Option {
	return func(o *options) {
		o.cleanLimit = value
	}
}

// WithLogLifeTime - устанавливает опцию cleanLimit для ComponentService.
func WithLogLifeTime(value time.Duration) Option {
	return func(o *options) {
		o.logLifeTime = value
	}
}

// WithTaskCleanRecordsOpts - устанавливает опцию taskCleanerOpts для ComponentService.
func WithTaskCleanRecordsOpts(value ...task.Option) Option {
	return func(o *options) {
		o.taskCleanerOpts = append(o.taskCleanerOpts, value...)
	}
}
