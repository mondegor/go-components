package consume

type (
	// Option - настройка объекта QueueConsumer.
	Option func(o *options)

	options struct {
		consumer *QueueConsumer
	}
)

// WithStorageCompleted - устанавливает опцию storageCompleted для QueueConsumer.
func WithStorageCompleted(value completedItemStorage) Option {
	return func(o *options) {
		o.consumer.storageCompleted = value
	}
}

// WithStorageCrashed - устанавливает опцию storageCrashed для QueueConsumer.
func WithStorageCrashed(value crashedItemStorage) Option {
	return func(o *options) {
		o.consumer.storageCrashed = value
	}
}
