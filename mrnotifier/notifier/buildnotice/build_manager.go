package buildnotice

import (
	"github.com/mondegor/go-components/mrnotifier/notifier/dto"
	templateentity "github.com/mondegor/go-components/mrnotifier/template/entity"
)

const (
	channelEmail     = "email"
	channelMessenger = "messenger"
	channelSMS       = "sms"
)

type (
	// BuildManager - собирает уведомление/уведомления с использованием шаблона
	// в email письмо для отправки получателям.
	BuildManager struct {
		mailBuilder      *MailBuilder
		messengerBuilder *messengerBuilder
		smsBuilder       *smsBuilder
	}

	noticeRenderer interface {
		Render(notice string, data map[string]string) (string, error)
	}
)

// NewBuildManager - создаёт объект BuildManager.
func NewBuildManager(noticeRenderer noticeRenderer) *BuildManager {
	return &BuildManager{
		mailBuilder:      newMailBuilder(noticeRenderer, channelEmail),
		messengerBuilder: newMessengerBuilder(noticeRenderer, channelMessenger),
		smsBuilder:       newSMSBuilder(noticeRenderer, channelSMS),
	}
}

// Build - возвращает подготовленные уведомления на основе переданных переменных и данных из шаблона уведомления.
func (p *BuildManager) Build(vars map[string]string, templ templateentity.TemplateData) ([]dto.Notice, error) {
	notices := make([]dto.Notice, 0, 4)

	if templ.Mail != nil && !templ.Mail.IsDisabled {
		list, err := p.mailBuilder.Build(vars, templ.Mail)
		if err != nil {
			return nil, err
		}

		notices = append(notices, list...)
	}

	if templ.Messenger != nil && !templ.Messenger.IsDisabled {
		list, err := p.messengerBuilder.Build(vars, templ.Messenger)
		if err != nil {
			return nil, err
		}

		notices = append(notices, list...)
	}

	if templ.SMS != nil && !templ.SMS.IsDisabled {
		list, err := p.smsBuilder.Build(vars, templ.SMS)
		if err != nil {
			return nil, err
		}

		notices = append(notices, list...)
	}

	return notices, nil
}
