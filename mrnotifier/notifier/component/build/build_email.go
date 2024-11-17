package build

import (
	"fmt"

	"github.com/mondegor/go-sysmess/mrmsg"

	"github.com/mondegor/go-components/mrmailer"
	"github.com/mondegor/go-components/mrmailer/dto"
	templaterentity "github.com/mondegor/go-components/mrnotifier/template/entity"
)

func (co *NoticeBuilder) buildEmail(vars map[string]string, mail *templaterentity.DataEmail) ([]dto.Message, error) {
	// в переменной mail.Content должна содержаться переменная
	// mrmailer.FieldPreHeader в которую подставится этот заголовок
	if mail.Preheader != "" {
		vars[mrmailer.FieldPreHeader] = mail.Preheader
	}

	subject, err := mrmsg.Render(mail.Subject, vars)
	if err != nil {
		return nil, fmt.Errorf("subject rendering failed: %w", err)
	}

	content, err := mrmsg.Render(mail.Content, vars)
	if err != nil {
		return nil, fmt.Errorf("content rendering failed: %w", err)
	}

	messages := make([]dto.Message, len(mail.ObserverEmails)+1)

	messages[0] = dto.Message{
		Channel: channelEmail,
		Data: dto.MessageData{
			Email: &dto.DataEmail{
				ContentType: co.contentType(mail.ContentType),
				From:        co.emailAddress(mrmailer.FieldFrom, vars, mail.From),
				To:          co.emailAddress(mrmailer.FieldTo, vars, mail.To),
				ReplyTo:     co.replyTo(vars, mail.ReplyTo),
				Subject:     subject,
				Content:     content,
			},
		},
	}

	for i := 1; i <= len(mail.ObserverEmails); i++ {
		messages[i] = messages[0]

		bodyCopy := *messages[0].Data.Email // копируется тело уведомления
		messages[i].Data.Email = &bodyCopy
		messages[i].Data.Email.To = mail.ObserverEmails[i-1] // заменяется получатель
	}

	return messages, nil
}

func (co *NoticeBuilder) contentType(value string) string {
	if value == "" {
		return mrmailer.ContentTypePlain
	}

	return value
}

func (co *NoticeBuilder) emailAddress(varName string, vars map[string]string, def *dto.EmailAddress) dto.EmailAddress {
	if vars[varName+".email"] != "" {
		return dto.EmailAddress{
			Name:  vars[varName+".name"], // OPTIONAL
			Email: vars[varName+".email"],
		}
	}

	if vars[varName] != "" {
		return dto.EmailAddress{
			Email: vars[varName],
		}
	}

	if def != nil {
		return *def
	}

	return dto.EmailAddress{}
}

func (co *NoticeBuilder) replyTo(vars map[string]string, def *dto.EmailAddress) *dto.EmailAddress {
	addr := co.emailAddress(mrmailer.FieldReplyTo, vars, def)

	if addr.Empty() {
		return nil
	}

	return &addr
}
