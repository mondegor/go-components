package clean

import (
	"time"

	"github.com/mondegor/go-components/mrqueue"
)

type (
	// Option - настройка объекта QueueCleaner.
	Option func(co *QueueCleaner)
)

// WithStorageCompleted - устанавливает опцию storageCompleted для QueueCleaner.
func WithStorageCompleted(value mrqueue.CompletedStorage) Option {
	return func(co *QueueCleaner) {
		co.storageCompleted = value
	}
}

// WithStorageBroken - устанавливает опцию storageBroken для QueueCleaner.
func WithStorageBroken(value mrqueue.BrokenStorage) Option {
	return func(co *QueueCleaner) {
		co.storageBroken = value
	}
}

// WithCompletedExpiry - устанавливает опцию completedExpiry для QueueCleaner.
func WithCompletedExpiry(value time.Duration) Option {
	return func(co *QueueCleaner) {
		co.completedExpiry = value
	}
}

// WithBrokenExpiry - устанавливает опцию brokenExpiry для QueueCleaner.
func WithBrokenExpiry(value time.Duration) Option {
	return func(co *QueueCleaner) {
		co.brokenExpiry = value
	}
}
