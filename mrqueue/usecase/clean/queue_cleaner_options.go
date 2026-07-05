package clean

import "context"

type (
	// Option - настройка объекта QueueCleaner.
	Option func(o *options)

	options struct {
		cleaner *QueueCleaner
	}
)

// WithAfterClean - устанавливает опцию afterCleanFunc для QueueCleaner.
func WithAfterClean(value func(ctx context.Context, itemsIDs []uint64) error) Option {
	return func(o *options) {
		o.cleaner.afterCleanFunc = value
	}
}
