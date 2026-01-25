package clean

import (
	"context"
	"time"
)

type (
	// Option - настройка объекта CrashedItemsCleaner.
	Option func(o *options)

	options struct {
		cleaner *CrashedItemsCleaner
	}
)

// WithExpiry - устанавливает опцию crashedExpiry для CrashedItemsCleaner.
func WithExpiry(value time.Duration) Option {
	return func(o *options) {
		o.cleaner.crashedExpiry = value
	}
}

// WithAfterClean - устанавливает опцию afterCleanFunc для CrashedItemsCleaner.
func WithAfterClean(value func(ctx context.Context, itemsIDs []uint64) error) Option {
	return func(o *options) {
		o.cleaner.afterCleanFunc = value
	}
}
