package action

import (
	"time"
)

const (
	defaultMaxAttempts   = 3
	defaultMaxResends    = 3
	defaultMinResendTime = 2 * time.Minute
	defaultExpiry        = 10 * time.Minute
)

type (
	// Option - настройка объекта MessageSender.
	Option func(o *confirmOptions)

	confirmOptions struct {
		maxAttempts   int16
		maxResends    int16
		minResendTime time.Duration
		expiry        time.Duration
	}
)

func newConfirmOptions(opts []Option) confirmOptions {
	o := confirmOptions{
		minResendTime: defaultMinResendTime,
	}

	for _, opt := range opts {
		opt(&o)
	}

	if o.maxAttempts < 1 {
		o.maxAttempts = defaultMaxAttempts
	}

	if o.maxResends < 1 {
		o.maxResends = defaultMaxResends
	}

	if o.expiry < 1 {
		o.expiry = defaultExpiry
	}

	return o
}

// WithMaxAttempts - устанавливает кол-во попыток отправки одного сообщения.
func WithMaxAttempts(value int16) Option {
	return func(o *confirmOptions) {
		o.maxAttempts = value
	}
}

// WithMaxResends - устанавливает кол-во повторных отправок кода подтверждения.
func WithMaxResends(value int16) Option {
	return func(o *confirmOptions) {
		o.maxResends = value
	}
}

// WithMinResendTime - устанавливает минимальную паузу между повторными отправками кода.
func WithMinResendTime(value time.Duration) Option {
	return func(o *confirmOptions) {
		o.minResendTime = value
	}
}

// WithExpiry - устанавливает срок жизни действия подтверждения.
func WithExpiry(value time.Duration) Option {
	return func(o *confirmOptions) {
		o.expiry = value
	}
}
