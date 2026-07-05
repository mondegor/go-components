package provider

import (
	"github.com/mondegor/go-core/mrtrace"

	"github.com/mondegor/go-components/mrmailer"
)

type (
	// Option - настройка объекта SenderProvider.
	Option func(o *options)

	options struct {
		sender *messageSender
		tracer mrtrace.Tracer
	}
)

// WithClientMail - устанавливает клиента, для возможности отправки электронных писем на почтовые сервисы.
func WithClientMail(value mrmailer.MessageSender) Option {
	return func(o *options) {
		o.sender.clientMail = value
	}
}

// WithClientMessenger - устанавливает клиента, для возможности отправки сообщений в Messenger сервис.
func WithClientMessenger(value mrmailer.MessageSender) Option {
	return func(o *options) {
		o.sender.clientMessenger = value
	}
}

// WithClientSMS - устанавливает клиента, для возможности отправки SMS сообщений на телефон.
func WithClientSMS(value mrmailer.MessageSender) Option {
	return func(o *options) {
		o.sender.clientSMS = value
	}
}

// WithTracer - устанавливает трейсинг отправки сообщений.
func WithTracer(tracer mrtrace.Tracer) Option {
	return func(o *options) {
		o.tracer = tracer
	}
}
