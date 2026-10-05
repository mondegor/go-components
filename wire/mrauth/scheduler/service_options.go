package scheduler

import (
	"time"

	"github.com/mondegor/go-core/mrprocess/job/task"
)

type (
	// Option - настройка объекта ComponentService.
	Option func(o *options)

	options struct {
		captionPrefix        string
		cleanLimit           int
		operationLogLifeTime time.Duration
		activityLogLifeTime  time.Duration
		securityLogLifeTime  time.Duration
		taskCleanerOpts      []task.Option
		taskTrimSessionsOpts []task.Option
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

// WithOperationLogLifeTime - устанавливает срок хранения записей журнала защищённых операций
// (0 - срок по умолчанию).
func WithOperationLogLifeTime(value time.Duration) Option {
	return func(o *options) {
		o.operationLogLifeTime = value
	}
}

// WithActivityLogLifeTime - устанавливает срок хранения записей журнала активности пользователей
// (0 - срок по умолчанию).
func WithActivityLogLifeTime(value time.Duration) Option {
	return func(o *options) {
		o.activityLogLifeTime = value
	}
}

// WithSecurityLogLifeTime - устанавливает срок хранения записей журнала безопасности пользователей
// (0 - срок по умолчанию).
func WithSecurityLogLifeTime(value time.Duration) Option {
	return func(o *options) {
		o.securityLogLifeTime = value
	}
}

// WithTaskCleanRecordsOpts - устанавливает опцию taskCleanerOpts для ComponentService.
func WithTaskCleanRecordsOpts(value ...task.Option) Option {
	return func(o *options) {
		o.taskCleanerOpts = append(o.taskCleanerOpts, value...)
	}
}

// WithTaskTrimSessionsOpts - устанавливает опцию taskTrimSessionsOpts
// (задача чистки лишних сессий) для ComponentService.
func WithTaskTrimSessionsOpts(value ...task.Option) Option {
	return func(o *options) {
		o.taskTrimSessionsOpts = append(o.taskTrimSessionsOpts, value...)
	}
}
