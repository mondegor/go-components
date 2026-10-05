package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrtype"

	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
)

type (
	// SecureOperationLog - запись журнала защищённых операций.
	SecureOperationLog struct {
		RecordID      int64
		VisitorID     uuid.UUID
		SourceName    string
		ConfirmMethod confirmmethod.Enum
		LogStatus     logstatus.Enum
		Reason        logreason.Enum
		ClientIP      mrtype.DetailedIP
		CreatedAt     time.Time
	}
)

// NewSecureOperationLog - создаёт запись журнала защищённых операций, фиксируя время наступления
// события: сохранение записи может быть отложено, поэтому время сохранения от него отличается.
func NewSecureOperationLog(
	visitorID uuid.UUID,
	clientIP mrtype.DetailedIP,
	sourceName string,
	confirmMethod confirmmethod.Enum,
	status logstatus.Enum,
	reason logreason.Enum,
) SecureOperationLog {
	return SecureOperationLog{
		VisitorID:     visitorID,
		SourceName:    sourceName,
		ConfirmMethod: confirmMethod,
		LogStatus:     status,
		Reason:        reason,
		ClientIP:      clientIP,
		CreatedAt:     time.Now().UTC(),
	}
}
