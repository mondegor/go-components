package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrtype"

	"github.com/mondegor/go-components/mrauth/enum/securityevent"
)

type (
	// SecurityLogEvent - запись журнала безопасности пользователя.
	SecurityLogEvent struct {
		RecordID  int64
		UserID    uuid.UUID
		EventType securityevent.Enum
		ClientIP  mrtype.DetailedIP
		UserAgent string
		Extra     *SecurityLogExtra
		CreatedAt time.Time
	}

	// SecurityLogExtra - подробности события журнала безопасности; у каждого типа события
	// заполнены только относящиеся к нему поля.
	SecurityLogExtra struct {
		OldValue  string `json:"old_value,omitempty"` // прежний адрес или номер
		NewValue  string `json:"new_value,omitempty"` // новый адрес или номер
		Factor    string `json:"factor,omitempty"`    // тип второго фактора (auth2fatype)
		Remaining *int   `json:"remaining,omitempty"` // остаток аварийных кодов
	}
)

// NewSecurityLogEvent - создаёт запись журнала безопасности. userAgent ожидается уже приведённым
// к безопасному виду (нормализуется на границе ввода, см. request.ParserClient).
func NewSecurityLogEvent(
	userID uuid.UUID,
	clientIP mrtype.DetailedIP,
	userAgent string,
	eventType securityevent.Enum,
	extra *SecurityLogExtra,
) SecurityLogEvent {
	return SecurityLogEvent{
		UserID:    userID,
		EventType: eventType,
		ClientIP:  clientIP,
		UserAgent: userAgent,
		Extra:     extra,
	}
}
