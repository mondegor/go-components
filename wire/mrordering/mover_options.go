package mrordering

import (
	"github.com/mondegor/go-core/mrstorage"
)

type (
	// Option - настройка объекта service.NodeMover.
	Option func(o *options)

	options struct {
		storageCondition mrstorage.SQLPartFunc
	}
)

// WithCondition - устанавливает дополнительное условие на список элементов, участвующих в сортировке.
func WithCondition(value mrstorage.SQLPartFunc) Option {
	return func(o *options) {
		o.storageCondition = value
	}
}
