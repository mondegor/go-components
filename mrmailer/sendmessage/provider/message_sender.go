package provider

import (
	"github.com/mondegor/go-components/mrmailer"
	"github.com/mondegor/go-components/mrmailer/entity"
	"github.com/mondegor/go-components/mrmailer/sendmessage"
)

type (
	// SenderProvider - обработчик сообщений с целью их отправки конечному получателю.
	messageSender struct {
		clientMail      mrmailer.MessageSender
		clientMessenger mrmailer.MessageSender
		clientSMS       mrmailer.MessageSender
	}
)

// New - создаёт объект messageSender.
func New(opts ...Option) sendmessage.SenderProvider {
	o := options{
		sender: &messageSender{},
	}

	for _, opt := range opts {
		opt(&o)
	}

	if o.tracer != nil {
		if o.sender.clientMail != nil {
			o.sender.clientMail = newTraceWrapper(o.tracer, "clientMail", o.sender.clientMail)
		}

		if o.sender.clientMessenger != nil {
			o.sender.clientMessenger = newTraceWrapper(o.tracer, "clientMessenger", o.sender.clientMessenger)
		}

		if o.sender.clientSMS != nil {
			o.sender.clientSMS = newTraceWrapper(o.tracer, "clientSMS", o.sender.clientSMS)
		}
	}

	return o.sender
}

// Sender - возвращает провайдера для отправки сообщений.
func (p *messageSender) Sender(data entity.MessageData) (mrmailer.MessageSender, error) {
	if data.Mail != nil {
		if p.clientMail == nil {
			return nil, mrmailer.ErrInternalProviderClientNotSpecified.New(
				"type", "mail",
			)
		}

		return p.clientMail, nil
	}

	if data.Messenger != nil {
		if p.clientMessenger == nil {
			return nil, mrmailer.ErrInternalProviderClientNotSpecified.New(
				"type", "messenger",
			)
		}

		return p.clientMessenger, nil
	}

	if data.SMS != nil {
		if p.clientSMS == nil {
			return nil, mrmailer.ErrInternalProviderClientNotSpecified.New(
				"type", "sms",
			)
		}

		return p.clientSMS, nil
	}

	return nil, mrmailer.ErrInternalProviderClientNotSpecified.New(
		"type", "unknown",
	)
}
