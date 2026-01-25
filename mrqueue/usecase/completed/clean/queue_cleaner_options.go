package clean

import (
	"context"
	"time"
)

type (
	// Option - настройка объекта CompletedItemsCleaner.
	Option func(o *options)

	options struct {
		cleaner *CompletedItemsCleaner
	}
)

// WithExpiry - устанавливает опцию completedExpiry для CompletedItemsCleaner.
func WithExpiry(value time.Duration) Option {
	return func(o *options) {
		o.cleaner.completedExpiry = value
	}
}

// WithAfterClean - устанавливает опцию afterCleanFunc для CompletedItemsCleaner.
func WithAfterClean(value func(ctx context.Context, itemsIDs []uint64) error) Option {
	return func(o *options) {
		o.cleaner.afterCleanFunc = value
	}
}
