package auth2fa

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth"
)

const (
	// notifyKeyRecoveryCodeUsed - ключ уведомления об использовании аварийного кода.
	notifyKeyRecoveryCodeUsed = "user.recovery_codes.used"
)

type (
	// RecoveryAlerter - оповещает пользователя через notifierAPI о каждом использовании
	// аварийного кода: сообщает остаток и признак low - остаток не выше threshold, при котором
	// в письме предлагается перевыпустить список. Получатель передаётся в props["to"]
	// как ID пользователя (uuid.UUID), поэтому notifierAPI должен разрешать его в адрес
	// (см. notify.UserEmailNotifier).
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

// SendAlert - оповещает пользователя об использовании аварийного кода и остатке кодов.
func (uc *RecoveryAlerter) SendAlert(ctx context.Context, userID uuid.UUID, codeRemaining int) error {
	return uc.notifierAPI.Send(
		ctx,
		notifyKeyRecoveryCodeUsed,
		conv.Group{
			"to":        userID,
			"remaining": codeRemaining,
			"low":       codeRemaining <= uc.threshold,
		},
	)
}
