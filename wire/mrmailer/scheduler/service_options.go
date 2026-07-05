package scheduler

import (
	"time"

	"github.com/mondegor/go-sysmess/mrprocess/job/task"
)

type (
	// Option - настройка объекта schedule.TaskScheduler.
	Option func(o *options)

	options struct {
		captionPrefix      string
		changeBatchSize    int
		changeRetryTimeout time.Duration
		changeRetryDelayed time.Duration
		cleanBatchSize     int
		taskChangerOpts    []task.Option
		taskCleanerOpts    []task.Option
	}
)

// WithCaptionPrefix - устанавливает опцию caption для schedule.TaskScheduler.
func WithCaptionPrefix(value string) Option {
	return func(o *options) {
		o.captionPrefix = value
	}
}

// WithChangeBatchSize - устанавливает опцию changeBatchSize для schedule.TaskScheduler.
func WithChangeBatchSize(value int) Option {
	return func(o *options) {
		o.changeBatchSize = value
	}
}

// WithChangeRetryTimeout - устанавливает опцию changeRetryTimeout для schedule.TaskScheduler.
func WithChangeRetryTimeout(value time.Duration) Option {
	return func(o *options) {
		o.changeRetryTimeout = value
	}
}

// WithChangeRetryDelayed - устанавливает опцию changeRetryDelayed для schedule.TaskScheduler.
func WithChangeRetryDelayed(value time.Duration) Option {
	return func(o *options) {
		o.changeRetryDelayed = value
	}
}

// WithCleanBatchSize - устанавливает опцию cleanBatchSize для schedule.TaskScheduler.
func WithCleanBatchSize(value int) Option {
	return func(o *options) {
		o.cleanBatchSize = value
	}
}

// WithTaskChangeFromToRetryOpts - устанавливает опцию taskChangerOpts для schedule.TaskScheduler.
func WithTaskChangeFromToRetryOpts(value ...task.Option) Option {
	return func(o *options) {
		o.taskChangerOpts = append(o.taskChangerOpts, value...)
	}
}

// WithTaskCleanMessagesOpts - устанавливает опцию taskCleanerOpts для schedule.TaskScheduler.
func WithTaskCleanMessagesOpts(value ...task.Option) Option {
	return func(o *options) {
		o.taskCleanerOpts = append(o.taskCleanerOpts, value...)
	}
}
