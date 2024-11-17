package handle

import (
	"github.com/mondegor/go-components/mrmailer"
)

type (
	// Option - настройка объекта MessageHandler.
	Option func(co *MessageHandler)
)

// WithClientEmail - устанавливает клиента, для возможности отправки электронных писем на почтовые сервисы.
func WithClientEmail(value mrmailer.MessageProvider) Option {
	return func(co *MessageHandler) {
		co.clientEmail = value
	}
}

// WithClientSMS - устанавливает клиента, для возможности отправки SMS сообщений на телефон.
func WithClientSMS(value mrmailer.MessageProvider) Option {
	return func(co *MessageHandler) {
		co.clientSMS = value
	}
}

// WithClientTelegram - устанавливает клиента, для возможности отправки сообщений в Telegram сервис.
func WithClientTelegram(value mrmailer.MessageProvider) Option {
	return func(co *MessageHandler) {
		co.clientTelegram = value
	}
}
