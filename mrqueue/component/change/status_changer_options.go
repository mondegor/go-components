package change

import (
	"time"

	"github.com/mondegor/go-components/mrqueue"
)

type (
	// Option - настройка объекта StatusChanger.
	Option func(co *StatusChanger)
)

// WithStorageBroken - устанавливает опцию storageBroken для StatusChanger.
func WithStorageBroken(value mrqueue.BrokenStorage) Option {
	return func(co *StatusChanger) {
		co.storageBroken = value
	}
}

// WithRetryTimeout - устанавливает опцию retryTimeout для StatusChanger.
func WithRetryTimeout(value time.Duration) Option {
	return func(co *StatusChanger) {
		co.retryTimeout = value
	}
}

// WithRetryDelayed - устанавливает опцию retryDelayed для StatusChanger.
func WithRetryDelayed(value time.Duration) Option {
	return func(co *StatusChanger) {
		co.retryDelayed = value
	}
}
