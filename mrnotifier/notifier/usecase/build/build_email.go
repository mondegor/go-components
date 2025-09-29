package build

import (
	"github.com/mondegor/go-sysmess/mrerr/mr"
	"github.com/mondegor/go-webcore/mrsender"

	"github.com/mondegor/go-components/mrmailer/dto"
	"github.com/mondegor/go-components/mrnotifier"
	templaterentity "github.com/mondegor/go-components/mrnotifier/template/entity"
)

func (co *NoticeBuilder) buildEmail(vars map[string]string, templMail *templaterentity.DataEmail) ([]dto.Message, error) {
	// в переменной templMail.Content должна содержаться переменная
	// mrnotifier.FieldPreHeader в которую подставится этот заголовок
	if templMail.Preheader != "" {
		vars[mrnotifier.FieldPreHeader] = templMail.Preheader
	}

	subject, err := co.messageRenderer.Render(templMail.Subject, vars) // TODO: временно
	if err != nil {
		return nil, mr.ErrInternal.Wrap(err, "details", "subject rendering failed")
	}

	content, err := co.messageRenderer.Render(templMail.Content, vars) // TODO: временно
	if err != nil {
		return nil, mr.ErrInternal.Wrap(err, "details", "content rendering failed")
	}

	messages := make([]dto.Message, len(templMail.ObserverEmails)+1)

	messages[0] = dto.Message{
		Channel: channelEmail,
		Data: dto.MessageData{
			Email: &dto.DataEmail{
				ContentType: co.contentType(templMail.ContentType),
				From:        co.emailAddressName(mrnotifier.FieldFromName, vars, templMail.FromName),
				To:          co.emailAddress(mrnotifier.FieldTo, vars, templMail.To),
				ReplyTo:     co.emailAddress(mrnotifier.FieldReplyTo, vars, templMail.ReplyTo),
				Subject:     subject,
				Content:     content,
			},
		},
	}

	if messages[0].Data.Email.To == "" {
		return nil, mr.ErrInternal.Wrap(err, "details", "field 'to' is empty")
	}

	for i := 1; i <= len(templMail.ObserverEmails); i++ {
		if templMail.ObserverEmails[i-1] == "" {
			continue
		}

		messages[i] = messages[0]

		bodyCopy := *messages[0].Data.Email // копируется уведомление
		messages[i].Data.Email = &bodyCopy
		messages[i].Data.Email.Subject = "[Copy] " + messages[0].Data.Email.Subject + " (" + messages[0].Data.Email.To + ")"
		messages[i].Data.Email.To = templMail.ObserverEmails[i-1] // заменяется получатель
	}

	return messages, nil
}

func (co *NoticeBuilder) contentType(value string) string {
	if value == "" {
		return mrsender.ContentTypePlain
	}

	return value
}

func (co *NoticeBuilder) emailAddressName(varName string, vars map[string]string, defaultAddressName string) string {
	if vars[varName] != "" {
		return vars[varName]
	}

	return defaultAddressName
}

func (co *NoticeBuilder) emailAddress(varName string, vars map[string]string, defaultAddress *string) string {
	if vars[varName] != "" {
		return vars[varName]
	}

	if defaultAddress != nil {
		return *defaultAddress
	}

	return ""
}
