package buildnotice

import (
	"github.com/mondegor/go-components/mrnotifier/notifier/dto"
	templateentity "github.com/mondegor/go-components/mrnotifier/template/entity"
)

type (
	// smsBuilder - собирает уведомление/уведомления с использованием шаблона
	// в smsBuilder сообщение для отправки получателям.
	smsBuilder struct {
		noticeRenderer noticeRenderer
		channel        string
	}
)

func newSMSBuilder(
	noticeRenderer noticeRenderer,
	channel string,
) *smsBuilder {
	return &smsBuilder{
		noticeRenderer: noticeRenderer,
		channel:        channel,
	}
}

// Build - возвращает подготовленные уведомления на основе переданных переменных и данных из шаблона уведомления.
func (b *smsBuilder) Build(_ map[string]string, _ *templateentity.DataSMS) ([]dto.Notice, error) {
	// TODO: требует реализации
	return nil, nil
}
