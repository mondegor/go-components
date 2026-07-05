package adapter

import (
	"context"
	"strings"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-webcore/mrclient"
	"github.com/mondegor/go-webcore/mrclient/mail"

	"github.com/mondegor/go-components/mrmailer"
	"github.com/mondegor/go-components/mrmailer/entity"
)

type (
	// Sender - провайдер для отправки сообщений через заданный мессенджер.
	mailSender struct {
		clientAPI        mrclient.MailSender
		defaultFrom      string
		defaultFromEmail string
	}
)

// NewMailSender - создаёт объект mailSender.
// В переменной defaultFromEmail обязателен для заполнения
// и в ней должен находиться электронный адрес отправителя, в том числе и расширенный.
func NewMailSender(
	clientAPI mrclient.MailSender,
	defaultFromEmail string,
) (mrmailer.MessageSender, error) {
	addr, err := mail.ParseAddress(defaultFromEmail)
	if err != nil {
		return nil, errors.WrapInternalError(
			err,
			"parsing variable failed",
			"defaultFromEmail", defaultFromEmail,
		)
	}

	return &mailSender{
		clientAPI:        clientAPI,
		defaultFrom:      addr.String(),
		defaultFromEmail: addr.Address,
	}, nil
}

// Send - отправляет указанное сообщение.
func (s *mailSender) Send(ctx context.Context, message entity.Message) error {
	if message.Data.Mail == nil {
		return errors.ErrInternalIncorrectInputData.WithDetails("message.Data.Mail is nil")
	}

	smtpMessage, err := mail.NewMessage(
		s.makeFromAddress(message.Data.Mail.From),
		message.Data.Mail.To,
		mail.WithContentType(message.Data.Mail.ContentType),
		mail.WithSubject(message.Data.Mail.Subject),
		mail.WithReplyTo(s.makeFromAddress(message.Data.Mail.ReplyTo)),
	)
	if err != nil {
		return err
	}

	err = s.clientAPI.SendMail(
		ctx,
		smtpMessage.From(),
		smtpMessage.To(),
		smtpMessage.Header(),
		message.Data.Mail.Content,
	)
	if err != nil {
		return err
	}

	return nil
}

func (s *mailSender) makeFromAddress(value string) string {
	if value == "" {
		return s.defaultFrom
	}

	// если в строке содержится email, то возвращается строка без изменений
	if strings.Contains(value, "@") {
		return value
	}

	// иначе в строке содержится только имя, поэтому к нему добавляется адрес отправителя
	return value + " <" + s.defaultFromEmail + ">"
}
