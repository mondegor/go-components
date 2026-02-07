package entity

import (
	"time"

	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
)

const (
	// ModelNameSecureOperation - название сущности.
	ModelNameSecureOperation = "mrauth.SecureOperation"
)

type (
	// SecureOperationLog - сообщение для получателя.
	SecureOperationLog struct {
		RecordID      uint64
		VisitorID     uuid.UUID
		OperationName string
		ConfirmMethod confirmmethod.Enum
		LogStatus     string // TODO: to status
		CreatedAt     time.Time
	}
)
