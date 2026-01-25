package produce

type (
	// Option - настройка объекта NoteProducer.
	Option func(o *options)

	options struct {
		producer *NoteProducer
	}
)

// WithRetryAttempts - устанавливает кол-во попыток отправки одного уведомления.
func WithRetryAttempts(value uint32) Option {
	return func(o *options) {
		o.producer.retryAttempts = value
	}
}
