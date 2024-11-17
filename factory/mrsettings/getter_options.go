package mrsettings

import (
	"github.com/mondegor/go-storage/mrstorage"

	"github.com/mondegor/go-components/mrsettings/features/fieldparser"
)

type (
	// GetterOption - настройка объекта get.SettingsGetter.
	GetterOption func(o *getterOptions)
)

// WithGetterFieldFormatterOpts - устанавливает опции парсинга данных поступающих из хранилища данных.
func WithGetterFieldFormatterOpts(value ...fieldparser.Option) GetterOption {
	return func(o *getterOptions) {
		if len(value) > 0 {
			o.fieldParser = append(o.fieldParser, value...)
		}
	}
}

// WithGetterCondition - устанавливает дополнительное условие на список элементов, участвующих в сортировке.
func WithGetterCondition(value mrstorage.SQLPartFunc) GetterOption {
	return func(o *getterOptions) {
		o.storageCondition = value
	}
}
