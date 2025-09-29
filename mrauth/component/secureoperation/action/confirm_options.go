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
	confirmOptions struct {
		maxAttempts   uint32
		maxResends    uint32
		minResendTime time.Duration
		expiry        time.Duration
	}

	// Option - настройка объекта MessageSender.
	Option func(co *confirmOptions)
)

func newConfirmOptions(opts []Option) confirmOptions {
	co := confirmOptions{
		maxAttempts:   defaultMaxAttempts,
		maxResends:    defaultMaxResends,
		minResendTime: defaultMinResendTime,
		expiry:        defaultExpiry,
	}

	for _, opt := range opts {
		opt(&co)
	}

	return co
}

// WithMaxAttempts - устанавливает кол-во попыток отправки одного сообщения.
func WithMaxAttempts(value uint32) Option {
	return func(co *confirmOptions) {
		if value > 0 {
			co.maxAttempts = value
		}
	}
}

// WithMaxResends - устанавливает поправку на задержку сообщения.
func WithMaxResends(value uint32) Option {
	return func(co *confirmOptions) {
		co.maxResends = value
	}
}

// WithMinResendTime - устанавливает поправку на задержку сообщения.
func WithMinResendTime(value time.Duration) Option {
	return func(co *confirmOptions) {
		co.minResendTime = value
	}
}

// WithExpiry - устанавливает поправку на задержку сообщения.
func WithExpiry(value time.Duration) Option {
	return func(co *confirmOptions) {
		if value > 0 {
			co.expiry = value
		}
	}
}
