package mrordering

import (
	"github.com/mondegor/go-storage/mrstorage"
)

type (
	// MoverOption - настройка объекта move.NodeMover.
	MoverOption func(o *moverOptions)
)

// WithCondition - устанавливает дополнительное условие на список элементов, участвующих в сортировке.
func WithCondition(value mrstorage.SQLPartFunc) MoverOption {
	return func(o *moverOptions) {
		o.storageCondition = value
	}
}
