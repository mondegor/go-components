package auth2fa

import (
	"context"

	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
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
	// (см. notify.UserEmailNotifier). Уведомление дополняется контекстом клиента,
	// предъявившего код (время, IP, устройство).
	RecoveryAlerter struct {
		notifierAPI mrauth.Notifier
		actorProps  actorPropsBuilder
		threshold   int
	}

	// actorPropsBuilder - дополняет props уведомления о событии безопасности контекстом клиента
	// (время события, IP, устройство).
	actorPropsBuilder interface {
		With(actor dto.ActorMeta, props conv.Group) conv.Group
	}
)

// NewRecoveryAlerter - создаёт объект RecoveryAlerter.
func NewRecoveryAlerter(notifierAPI mrauth.Notifier, actorProps actorPropsBuilder, threshold int) *RecoveryAlerter {
	return &RecoveryAlerter{
		notifierAPI: notifierAPI,
		actorProps:  actorProps,
		threshold:   threshold,
	}
}

// SendAlert - оповещает пользователя actor.UserID об использовании аварийного кода и остатке кодов.
func (uc *RecoveryAlerter) SendAlert(ctx context.Context, actor dto.ActorMeta, codeRemaining int) error {
	return uc.notifierAPI.Send(
		ctx,
		notifyKeyRecoveryCodeUsed,
		uc.actorProps.With(actor, conv.Group{
			"to":        actor.UserID,
			"remaining": codeRemaining,
			"low":       codeRemaining <= uc.threshold,
		}),
	)
}
