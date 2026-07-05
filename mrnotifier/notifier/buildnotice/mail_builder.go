package buildnotice

import (
	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrnotifier"
	"github.com/mondegor/go-components/mrnotifier/notifier/dto"
	templateentity "github.com/mondegor/go-components/mrnotifier/template/entity"
)

type (
	// MailBuilder - собирает уведомление/уведомления с использованием шаблона
	// в email письмо для отправки получателям.
	MailBuilder struct {
		noticeRenderer noticeRenderer
		channel        string
	}
)

// newMailBuilder - создаёт объект MailBuilder.
func newMailBuilder(
	noticeRenderer noticeRenderer,
	channel string,
) *MailBuilder {
	return &MailBuilder{
		noticeRenderer: noticeRenderer,
		channel:        channel,
	}
}

// Build - возвращает подготовленные уведомления на основе переданных переменных и данных из шаблона уведомления.
func (b *MailBuilder) Build(vars map[string]string, templMail *templateentity.DataMail) ([]dto.Notice, error) {
	// в переменной templMail.Content должна содержаться переменная
	// mrnotifier.FieldPreHeader в которую подставится этот заголовок
	if templMail.Preheader != "" {
		vars[mrnotifier.FieldPreHeader] = templMail.Preheader
	}

	subject, err := b.noticeRenderer.Render(templMail.Subject, vars) // TODO: временно
	if err != nil {
		return nil, errors.WrapInternalError(err, "subject rendering failed")
	}

	content, err := b.noticeRenderer.Render(templMail.Content, vars) // TODO: временно
	if err != nil {
		return nil, errors.WrapInternalError(err, "content rendering failed")
	}

	notices := make([]dto.Notice, len(templMail.ObserverEmails)+1)

	notices[0] = dto.Notice{
		Channel: b.channel,
		Data: dto.NoticeData{
			Mail: &dto.DataMail{
				ContentType: templMail.ContentType,
				From:        b.emailAddressName(mrnotifier.FieldFromName, vars, templMail.FromName),
				To:          b.emailAddress(mrnotifier.FieldTo, vars, templMail.To),
				ReplyTo:     b.emailAddress(mrnotifier.FieldReplyTo, vars, templMail.ReplyTo),
				Subject:     subject,
				Content:     content,
			},
		},
	}

	if notices[0].Data.Mail.To == "" {
		return nil, errors.NewInternalError("field 'to' is empty")
	}

	for i := 1; i <= len(templMail.ObserverEmails); i++ {
		if templMail.ObserverEmails[i-1] == "" {
			continue
		}

		notices[i] = notices[0]

		bodyCopy := *notices[0].Data.Mail // копируется уведомление
		notices[i].Data.Mail = &bodyCopy
		notices[i].Data.Mail.Subject = "[Copy] " + notices[0].Data.Mail.Subject + " (" + notices[0].Data.Mail.To + ")"
		notices[i].Data.Mail.To = templMail.ObserverEmails[i-1] // заменяется получатель
	}

	return notices, nil
}

func (b *MailBuilder) emailAddressName(varName string, vars map[string]string, defaultAddressName string) string {
	if vars[varName] != "" {
		return vars[varName]
	}

	return defaultAddressName
}

func (b *MailBuilder) emailAddress(varName string, vars map[string]string, defaultAddress *string) string {
	if vars[varName] != "" {
		return vars[varName]
	}

	if defaultAddress != nil {
		return *defaultAddress
	}

	return ""
}
