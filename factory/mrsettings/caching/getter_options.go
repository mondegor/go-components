package caching

import (
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-webcore/mrworker/job/task"

	"github.com/mondegor/go-components/mrsettings/features/fieldparser"
)

type (
	// GetterOption - настройка объекта cacheget.SettingsGetter.
	GetterOption func(o *getterOptions)
)

// WithFieldFormatterOpts - устанавливает опции парсинга данных поступающих из хранилища данных.
func WithFieldFormatterOpts(value ...fieldparser.Option) GetterOption {
	return func(o *getterOptions) {
		if len(value) > 0 {
			o.fieldParser = append(o.fieldParser, value...)
		}
	}
}

// WithTaskReloadSettingsOpts - устанавливает опции для обновления настроек из БД.
func WithTaskReloadSettingsOpts(value ...task.Option) GetterOption {
	return func(o *getterOptions) {
		if len(value) > 0 {
			o.taskReloadSettings = append(o.taskReloadSettings, value...)
		}
	}
}

// WithCondition - устанавливает дополнительное условие на список элементов, участвующих в сортировке.
func WithCondition(value mrstorage.SQLPartFunc) GetterOption {
	return func(o *getterOptions) {
		o.storageCondition = value
	}
}
