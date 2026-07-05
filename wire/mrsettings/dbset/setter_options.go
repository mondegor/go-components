package dbset

import (
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrsettings/field/format"
)

type (
	// Option - настройка объекта service.SettingsSetter.
	Option func(o *options)

	options struct {
		formatterOpts    []format.Option
		storageCondition mrstorage.SQLPartFunc
	}
)

// WithFieldFormatterOpts - устанавливает опции форматирования данных поступающих из внешнего источника.
func WithFieldFormatterOpts(value ...format.Option) Option {
	return func(o *options) {
		o.formatterOpts = append(o.formatterOpts, value...)
	}
}

// WithCondition - устанавливает дополнительное условие на список элементов, участвующих в сортировке.
func WithCondition(value mrstorage.SQLPartFunc) Option {
	return func(o *options) {
		o.storageCondition = value
	}
}
