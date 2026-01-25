package toretry

import "time"

type (
	// Option - настройка объекта ProcessingToRetryChanger.
	Option func(o *options)

	options struct {
		changer *ProcessingToRetryChanger
	}
)

// WithRetryTimeout - устанавливает опцию retryTimeout для ProcessingToRetryChanger.
func WithRetryTimeout(value time.Duration) Option {
	return func(o *options) {
		o.changer.retryTimeout = value
	}
}

// WithStorageCrashed - устанавливает опцию storageCrashed для ProcessingToRetryChanger.
func WithStorageCrashed(value crashedItemStorage) Option {
	return func(o *options) {
		o.changer.storageCrashed = value
	}
}
