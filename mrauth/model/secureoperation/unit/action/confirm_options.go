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
		maxAttempts   uint32
		maxResends    uint32
		minResendTime time.Duration
		expiry        time.Duration
	}
)

func newConfirmOptions(opts []Option) confirmOptions {
	o := confirmOptions{
		minResendTime: defaultMinResendTime,
		expiry:        defaultExpiry,
	}

	for _, opt := range opts {
		opt(&o)
	}

	if o.maxAttempts < 1 {
		o.maxAttempts = defaultMaxAttempts
	}

	if o.maxResends < 1 {
		o.maxAttempts = defaultMaxResends
	}

	return o
}

// WithMaxAttempts - устанавливает кол-во попыток отправки одного сообщения.
func WithMaxAttempts(value uint32) Option {
	return func(o *confirmOptions) {
		o.maxAttempts = value
	}
}

// WithMaxResends - устанавливает поправку на задержку сообщения.
func WithMaxResends(value uint32) Option {
	return func(o *confirmOptions) {
		o.maxResends = value
	}
}

// WithMinResendTime - устанавливает поправку на задержку сообщения.
func WithMinResendTime(value time.Duration) Option {
	return func(o *confirmOptions) {
		o.minResendTime = value
	}
}

// WithExpiry - устанавливает поправку на задержку сообщения.
func WithExpiry(value time.Duration) Option {
	return func(o *confirmOptions) {
		o.expiry = value
	}
}
