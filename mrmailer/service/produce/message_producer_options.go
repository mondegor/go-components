package produce

import "time"

type (
	// Option - настройка объекта MessageProducer.
	Option func(o *options)

	options struct {
		sender *MessageProducer
	}
)

// WithRetryAttempts - устанавливает кол-во попыток отправки одного сообщения.
func WithRetryAttempts(value uint32) Option {
	return func(o *options) {
		o.sender.retryAttempts = value
	}
}

// WithDelayCorrection - устанавливает поправку на задержку сообщения
// (чтобы учесть, что какое-то время сообщение будет находиться в очереди).
func WithDelayCorrection(value time.Duration) Option {
	return func(o *options) {
		o.sender.delayCorrection = value
	}
}
