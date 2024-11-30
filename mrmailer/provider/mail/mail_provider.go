package mail

import (
	"context"
	"strings"

	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrlog"
	"github.com/mondegor/go-webcore/mrsender"
	"github.com/mondegor/go-webcore/mrsender/mail"

	"github.com/mondegor/go-components/mrmailer/entity"
)

const (
	mailProviderName = "MailSender"
)

type (
	// Provider - провайдер для отправки сообщений через заданный мессенджер.
	Provider struct {
		mailerAPI        mrsender.MailProvider
		defaultFromEmail string
	}
)

// New - создаёт объект Provider.
// В переменной defaultFromEmail обязателен для заполнения
// и в ней должен находиться email (расширенный адрес не допускается).
func New(mailAPI mrsender.MailProvider, defaultFromEmail string) *Provider {
	return &Provider{
		mailerAPI:        mailAPI,
		defaultFromEmail: defaultFromEmail,
	}
}

// Send - отправляет указанное сообщение.
func (p *Provider) Send(ctx context.Context, message entity.Message) error {
	if message.Data.Email == nil {
		return mrcore.ErrUseCaseIncorrectInputData.New("message.Data.Email", "nil")
	}

	msg, err := mail.NewMessage(
		p.makeFromAddress(message.Data.Email.From),
		message.Data.Email.To,
		mail.WithSubject(message.Data.Email.Subject),
		mail.WithReplyTo(p.makeFromAddress(message.Data.Email.ReplyTo)),
	)
	if err != nil {
		return err
	}

	if err = p.mailerAPI.SendMail(ctx, msg.From(), msg.To(), msg.Header(), message.Data.Email.Content); err != nil {
		return err
	}

	mrlog.Ctx(ctx).
		Trace().
		Str("source", mailProviderName).
		Int64("messageId", int64(message.ID)).
		Str("channel", message.Channel).
		Send()

	return nil
}

func (p *Provider) makeFromAddress(value string) string {
	if value == "" {
		return p.defaultFromEmail
	}

	// если в строке содержится email, то возвращается строка без изменений
	if strings.Contains(value, "@") {
		return value
	}

	// иначе в строке содержится только имя, поэтому к нему добавляется адрес отправителя
	return value + " <" + p.defaultFromEmail + ">"
}
