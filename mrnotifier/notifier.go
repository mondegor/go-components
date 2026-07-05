package mrnotifier

import (
	"context"

	"github.com/mondegor/go-core/mrapp"
	"github.com/mondegor/go-core/mrtrace"

	"github.com/mondegor/go-components/mrnotifier/notifier/dto"
)

const (
	// ConfigDelayTime - время после которого уведомление должно быть отправлено / период задержки уведомления.
	ConfigDelayTime = "config.delayTime"

	// HeaderPrefix - префикс названий переменных уведомления, предназначенных для хранения в заголовке.
	HeaderPrefix = "header."

	// HeaderLang - название переменной языка уведомления.
	HeaderLang = HeaderPrefix + mrapp.KeyLangCode

	// HeaderCorrelationID - название переменной заголовка, содержащего CorrelationID.
	HeaderCorrelationID = HeaderPrefix + mrtrace.KeyCorrelationID

	// FieldFromName - имя отправителя (адрес подставится тот, с которого произойдёт отправка письма).
	FieldFromName = "fromName"

	// FieldTo - адрес получателя.
	FieldTo = "to"

	// FieldReplyTo - адрес для ответа на письмо.
	FieldReplyTo = "replyTo"

	// FieldPreHeader - дополнительный заголовок отображаемый в некоторых почтовых клиентах.
	FieldPreHeader = "preheader"
)

type (
	// NoteProducer - размещает данные об уведомлении в очереди для его сборки и отправки.
	NoteProducer interface {
		Send(ctx context.Context, key string, props map[string]any) error
	}

	// NoticeSender - занимается непосредственной отправкой сформированных уведомлений получателям.
	NoticeSender interface {
		Send(ctx context.Context, notices []dto.Notice) error
	}
)
