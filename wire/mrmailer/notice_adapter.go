package mrmailer

import (
	"context"

	"github.com/mondegor/go-components/mrmailer/dto"
	noticedto "github.com/mondegor/go-components/mrnotifier/notifier/dto"
)

type (
	// NoticeToMessageAdapterFunc - адаптер преобразовывает уведомления
	// в формат сообщений для их отправки с помощью mrmailer.
	NoticeToMessageAdapterFunc func(ctx context.Context, message ...dto.Message) error
)

// Send - отправляет уведомления в виде сообщений.
func (f NoticeToMessageAdapterFunc) Send(ctx context.Context, notices []noticedto.Notice) error {
	messages := make([]dto.Message, 0, len(notices))

	for _, notice := range notices {
		message := dto.Message{
			Channel:       notice.Channel,
			SendAfter:     notice.SendAfter,
			RetryAttempts: notice.RetryAttempts,
			Data: dto.MessageData{
				Header: notice.Data.Header,
			},
		}

		if notice.Data.Mail != nil {
			message.Data.Mail = &dto.DataMail{
				ContentType: notice.Data.Mail.ContentType,
				From:        notice.Data.Mail.From,
				To:          notice.Data.Mail.To,
				ReplyTo:     notice.Data.Mail.ReplyTo,
				Subject:     notice.Data.Mail.Subject,
				Content:     notice.Data.Mail.Content,
			}
		}

		if notice.Data.Messenger != nil {
			message.Data.Messenger = &dto.DataMessenger{
				From:    notice.Data.Messenger.From,
				ChatID:  notice.Data.Messenger.ChatID,
				Content: notice.Data.Messenger.Content,
			}
		}

		if notice.Data.SMS != nil {
			message.Data.SMS = &dto.DataSMS{
				From:    notice.Data.SMS.From,
				Phone:   notice.Data.SMS.Phone,
				Content: notice.Data.SMS.Content,
			}
		}

		messages = append(messages, message)
	}

	return f(ctx, messages...)
}
