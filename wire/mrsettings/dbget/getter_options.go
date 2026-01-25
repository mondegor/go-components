package dbget

import (
	"github.com/mondegor/go-storage/mrstorage"

	"github.com/mondegor/go-components/mrsettings/field/parse"
)

type (
	// Option - настройка объекта service.SettingsGetter.
	Option func(o *options)

	options struct {
		parserOpts       []parse.Option
		storageCondition mrstorage.SQLPartFunc
	}
)

// WithFieldParserOpts - устанавливает опции парсинга данных поступающих из хранилища данных.
func WithFieldParserOpts(value ...parse.Option) Option {
	return func(o *options) {
		o.parserOpts = append(o.parserOpts, value...)
	}
}

// WithCondition - устанавливает дополнительное условие на список элементов, участвующих в сортировке.
func WithCondition(value mrstorage.SQLPartFunc) Option {
	return func(o *options) {
		o.storageCondition = value
	}
}
