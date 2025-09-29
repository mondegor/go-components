package consume

import (
	"github.com/mondegor/go-components/mrqueue"
)

type (
	// Option - настройка объекта QueueConsumer.
	Option func(co *QueueConsumer)
)

// WithStorageCompleted - устанавливает опцию storageCompleted для QueueConsumer.
func WithStorageCompleted(value mrqueue.CompletedStorage) Option {
	return func(co *QueueConsumer) {
		co.storageCompleted = value
	}
}

// WithStorageBroken - устанавливает опцию storageBroken для QueueConsumer.
func WithStorageBroken(value mrqueue.BrokenStorage) Option {
	return func(co *QueueConsumer) {
		co.storageBroken = value
	}
}
