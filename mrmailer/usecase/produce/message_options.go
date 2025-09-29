package produce

import "time"

type (
	// Option - настройка объекта MessageSender.
	Option func(co *MessageSender)
)

// WithRetryAttempts - устанавливает кол-во попыток отправки одного сообщения.
func WithRetryAttempts(value uint32) Option {
	return func(co *MessageSender) {
		co.retryAttempts = value
	}
}

// WithDelayCorrection - устанавливает поправку на задержку сообщения
// (чтобы учесть, что какое-то время сообщение будет находиться в очереди).
func WithDelayCorrection(value time.Duration) Option {
	return func(co *MessageSender) {
		if value >= 0 {
			co.delayCorrection = value
		}
	}
}
