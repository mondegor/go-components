package mail

import (
	"context"
	"net/mail"
	"strings"

	"github.com/mondegor/go-sysmess/mrerr/mr"
	"github.com/mondegor/go-sysmess/mrtrace"
	"github.com/mondegor/go-webcore/mrsender"
	msg "github.com/mondegor/go-webcore/mrsender/mail"

	"github.com/mondegor/go-components/mrmailer/entity"
)

const (
	mailProviderName = "MailSender"
)

type (
	// Provider - провайдер для отправки сообщений через заданный мессенджер.
	Provider struct {
		mailerAPI        mrsender.MailProvider
		tracer           mrtrace.Tracer
		defaultFrom      string
		defaultFromEmail string
	}
)

// New - создаёт объект Provider.
// В переменной defaultFromEmail обязателен для заполнения
// и в ней должен находиться электронный адрес отправителя, в том числе и расширенный.
func New(mailAPI mrsender.MailProvider, tracer mrtrace.Tracer, defaultFromEmail string) (*Provider, error) {
	addr, err := mail.ParseAddress(defaultFromEmail)
	if err != nil {
		return nil, mr.ErrInternal.Wrap(err, "details", "parsing variable failed", "defaultFromEmail", defaultFromEmail)
	}

	return &Provider{
		mailerAPI:        mailAPI,
		tracer:           tracer,
		defaultFrom:      addr.String(),
		defaultFromEmail: addr.Address,
	}, nil
}

// Send - отправляет указанное сообщение.
func (p *Provider) Send(ctx context.Context, message entity.Message) error {
	if message.Data.Email == nil {
		return mr.ErrUseCaseIncorrectInternalInputData.New("reason", "message.Data.Email is nil")
	}

	p.tracer.Trace(
		ctx,
		"source", mailProviderName,
		"messageId", message.ID,
		"channel", message.Channel,
	)

	smtpMessage, err := msg.NewMessage(
		p.makeFromAddress(message.Data.Email.From),
		message.Data.Email.To,
		msg.WithSubject(message.Data.Email.Subject),
		msg.WithReplyTo(p.makeFromAddress(message.Data.Email.ReplyTo)),
	)
	if err != nil {
		return err
	}

	err = p.mailerAPI.SendMail(
		ctx,
		smtpMessage.From(),
		smtpMessage.To(),
		smtpMessage.Header(),
		message.Data.Email.Content,
	)
	if err != nil {
		return err
	}

	return nil
}

func (p *Provider) makeFromAddress(value string) string {
	if value == "" {
		return p.defaultFrom
	}

	// если в строке содержится email, то возвращается строка без изменений
	if strings.Contains(value, "@") {
		return value
	}

	// иначе в строке содержится только имя, поэтому к нему добавляется адрес отправителя
	return value + " <" + p.defaultFromEmail + ">"
}
