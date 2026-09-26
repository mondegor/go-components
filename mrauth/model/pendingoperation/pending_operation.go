package pendingoperation

import (
	"time"

	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
)

type (
	// PendingOperation - действующая операция личного кабинета пользователя, ожидающая
	// подтверждения либо применения. NewEmail - у операций смены емаила (оба шага), NewPhone -
	// у смены телефона; CurrentAction - только у неподтверждённой операции.
	PendingOperation struct {
		Token         string
		Type          operationtype.Enum
		Status        operationstatus.Enum
		ExpiresAt     time.Time
		NewEmail      string
		NewPhone      uint64
		CurrentAction *PendingAction
	}

	// PendingAction - текущее звено неподтверждённой операции. RemainingResends и ResendsAt
	// заданы, только если звено допускает повторную отправку кода.
	PendingAction struct {
		Method            confirmmethod.Enum
		RemainingAttempts int16
		RemainingResends  *int16
		ResendsAt         *time.Time
	}
)
