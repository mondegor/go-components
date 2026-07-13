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
		RecordID      uint64
		VisitorID     uuid.UUID
		OperationName string
		ConfirmMethod confirmmethod.Enum
		LogStatus     logstatus.Enum
		Reason        logreason.Enum
		ClientIP      mrtype.DetailedIP
		CreatedAt     time.Time
	}
)

// NewSecureOperationLog - создаёт запись журнала защищённых операций, фиксируя время наступления
// события: запись сохраняется в БД асинхронно (пачками), поэтому время вставки от него отличается.
func NewSecureOperationLog(
	visitorID uuid.UUID,
	clientIP mrtype.DetailedIP,
	operationName string,
	confirmMethod confirmmethod.Enum,
	status logstatus.Enum,
	reason logreason.Enum,
) SecureOperationLog {
	return SecureOperationLog{
		VisitorID:     visitorID,
		OperationName: operationName,
		ConfirmMethod: confirmMethod,
		LogStatus:     status,
		Reason:        reason,
		ClientIP:      clientIP,
		CreatedAt:     time.Now(),
	}
}
