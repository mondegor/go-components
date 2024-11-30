package mrnotifier

import (
	"context"

	"github.com/mondegor/go-components/mrmailer"
	"github.com/mondegor/go-components/mrmailer/dto"
	"github.com/mondegor/go-components/mrnotifier/notifier/entity"
)

const (
	// ConfigDelayTime - время после которого уведомление должно быть отправлено / период задержки уведомления.
	ConfigDelayTime = "config.delayTime"

	// HeaderPrefix - префикс названий переменных уведомления, предназначенных для хранения в заголовке.
	HeaderPrefix = "header."

	// HeaderLang - название переменной языка уведомления.
	HeaderLang = HeaderPrefix + mrmailer.HeaderLang

	// HeaderCorrelationID - название переменной заголовка, содержащего CorrelationID.
	HeaderCorrelationID = HeaderPrefix + mrmailer.HeaderCorrelationID

	FieldFromName  = "fromName"  // FieldFromName - имя отправителя (адрес подставится тот, с которого произойдёт отправка письма)
	FieldTo        = "to"        // FieldTo - адрес получателя
	FieldReplyTo   = "replyTo"   // FieldReplyTo - адрес для ответа на письмо
	FieldPreHeader = "preheader" // FieldPreHeader - дополнительный заголовок отображаемый в некоторых почтовых клиентах
)

type (
	// NoticeProducer - размещает уведомления в очереди для их сборки и отправки.
	NoticeProducer interface {
		SendNotice(ctx context.Context, key string, props map[string]any) error
	}

	// NoticeBuilder - собирает уведомление в форматированный вид для отправки их получателю.
	NoticeBuilder interface {
		BuildNotice(ctx context.Context, notice entity.Notice) (messages []dto.Message, err error)
	}

	// MailerAPI - отправляет внешним клиентом уведомление преобразованное в сообщение.
	MailerAPI interface {
		SendMessage(ctx context.Context, message dto.Message) error
		Send(ctx context.Context, messages []dto.Message) error
	}

	// NoticeStorage - предоставляет доступ к хранилищу уведомлений.
	NoticeStorage interface {
		FetchByIDs(ctx context.Context, rowsIDs []uint64) ([]entity.Notice, error)
		Insert(ctx context.Context, row entity.Notice) error
		DeleteByIDs(ctx context.Context, rowsIDs []uint64) error
	}
)
