package cacheget

import (
	"github.com/mondegor/go-core/mrprocess/job/task"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrsettings/field/parse"
)

type (
	// Option - настройка объекта caching.SettingsGetter.
	Option func(o *options)

	options struct {
		captionPrefix    string
		parserOpts       []parse.Option
		taskReloaderOpts []task.Option
		storageCondition mrstorage.SQLPartFunc
	}
)

// WithCaptionPrefix - устанавливает опцию captionPrefix для caching.SettingsGetter.
func WithCaptionPrefix(value string) Option {
	return func(o *options) {
		o.captionPrefix = value
	}
}

// WithFieldParserOpts - устанавливает опции парсинга данных поступающих из хранилища данных.
func WithFieldParserOpts(value ...parse.Option) Option {
	return func(o *options) {
		o.parserOpts = append(o.parserOpts, value...)
	}
}

// WithTaskReloadSettingsOpts - устанавливает опции для обновления настроек из БД.
func WithTaskReloadSettingsOpts(value ...task.Option) Option {
	return func(o *options) {
		o.taskReloaderOpts = append(o.taskReloaderOpts, value...)
	}
}

// WithCondition - устанавливает дополнительное условие на список элементов, участвующих в сортировке.
func WithCondition(value mrstorage.SQLPartFunc) Option {
	return func(o *options) {
		o.storageCondition = value
	}
}
