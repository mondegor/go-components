package senderrouter

import (
	"github.com/mondegor/go-components/mrmailer"
	"github.com/mondegor/go-components/mrmailer/entity"
)

type (
	// Router - выбирает отправителя по типу данных сообщения (mail, messenger, sms).
	Router struct {
		clientMail      mrmailer.MessageSender
		clientMessenger mrmailer.MessageSender
		clientSMS       mrmailer.MessageSender
	}
)

// New - создаёт объект Router.
func New(opts ...Option) *Router {
	o := options{
		router: &Router{},
	}

	for _, opt := range opts {
		opt(&o)
	}

	if o.tracer != nil {
		if o.router.clientMail != nil {
			o.router.clientMail = newTraceWrapper(o.tracer, "clientMail", o.router.clientMail)
		}

		if o.router.clientMessenger != nil {
			o.router.clientMessenger = newTraceWrapper(o.tracer, "clientMessenger", o.router.clientMessenger)
		}

		if o.router.clientSMS != nil {
			o.router.clientSMS = newTraceWrapper(o.tracer, "clientSMS", o.router.clientSMS)
		}
	}

	return o.router
}

// Sender - возвращает отправителя, соответствующего каналу сообщения.
func (p *Router) Sender(data entity.MessageData) (mrmailer.MessageSender, error) {
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
