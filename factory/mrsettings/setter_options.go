package mrsettings

import (
	"github.com/mondegor/go-storage/mrstorage"

	"github.com/mondegor/go-components/mrsettings/features/fieldformatter"
)

type (
	// SetterOption - настройка объекта set.SettingsSetter.
	SetterOption func(o *setterOptions)
)

// WithSetterFieldFormatterOpts - устанавливает опции форматирования данных поступающих из внешнего источника.
func WithSetterFieldFormatterOpts(value ...fieldformatter.Option) SetterOption {
	return func(o *setterOptions) {
		if len(value) > 0 {
			o.fieldFormatter = append(o.fieldFormatter, value...)
		}
	}
}

// WithSetterCondition - устанавливает дополнительное условие на список элементов, участвующих в сортировке.
func WithSetterCondition(value mrstorage.SQLPartFunc) SetterOption {
	return func(o *setterOptions) {
		o.storageCondition = value
	}
}
