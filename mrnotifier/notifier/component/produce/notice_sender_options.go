package produce

type (
	// Option - настройка объекта NoticeSender.
	Option func(co *NoticeSender)
)

// WithRetryAttempts - устанавливает кол-во попыток отправки одного уведомления.
func WithRetryAttempts(value uint32) Option {
	return func(co *NoticeSender) {
		co.retryAttempts = value
	}
}
