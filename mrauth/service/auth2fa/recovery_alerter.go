package auth2fa

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth"
)

const (
	// notifyKeyRecoveryCodesLow - ключ уведомления о низком остатке аварийных кодов.
	notifyKeyRecoveryCodesLow = "user.recovery_codes.low"
)

type (
	// RecoveryAlerter - отправляет пользователю уведомление о низком остатке аварийных
	// кодов через notifierAPI. Уведомление шлётся, только когда остаток упал до threshold и ниже.
	RecoveryAlerter struct {
		notifierAPI mrauth.Notifier
		threshold   int
	}
)

// NewRecoveryAlerter - создаёт объект RecoveryAlerter.
func NewRecoveryAlerter(notifierAPI mrauth.Notifier, threshold int) *RecoveryAlerter {
	return &RecoveryAlerter{
		notifierAPI: notifierAPI,
		threshold:   threshold,
	}
}

// SendAlert - уведомляет пользователя о низком остатке аварийных кодов, если остаток
// не превышает порог; иначе ничего не делает.
func (uc *RecoveryAlerter) SendAlert(ctx context.Context, userID uuid.UUID, codeRemaining int) error {
	if codeRemaining > uc.threshold {
		return nil
	}

	return uc.notifierAPI.Send(
		ctx,
		notifyKeyRecoveryCodesLow,
		conv.Group{
			"to":        userID,
			"remaining": codeRemaining,
		},
	)
}
