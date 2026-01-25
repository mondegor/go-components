package toready

import (
	"time"
)

type (
	// Option - настройка объекта RetryToReadyChanger.
	Option func(o *options)

	options struct {
		changer *RetryToReadyChanger
	}
)

// WithRetryDelayed - устанавливает опцию retryDelayed для RetryToReadyChanger.
func WithRetryDelayed(value time.Duration) Option {
	return func(o *options) {
		o.changer.retryDelayed = value
	}
}
