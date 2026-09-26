package handler

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// Disable2FA - обработчик отключения 2FA пользователя.
	Disable2FA struct {
		txManager    mrstorage.DBTxManager
		storage      user2faDisabler
		revoker      operationRevoker
		notifierAPI  mrnotifier.NoteProducer
		errorWrapper errors.Wrapper
	}

	user2faDisabler interface {
		Delete(ctx context.Context, userID uuid.UUID) error
	}

	// operationRevoker - отзывает все незавершённые операции пользователя.
	operationRevoker interface {
		RevokeAll(ctx context.Context, actor dto.ActorMeta, reason logreason.Enum) error
	}
)

// NewDisable2FA - создаёт объект Disable2FA.
func NewDisable2FA(
	txManager mrstorage.DBTxManager,
	storage user2faDisabler,
	revoker operationRevoker,
	notifierAPI mrnotifier.NoteProducer,
) *Disable2FA {
	return &Disable2FA{
		txManager:    txManager,
		storage:      storage,
		revoker:      revoker,
		notifierAPI:  notifierAPI,
		errorWrapper: errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - применяет подтверждённую операцию отключения 2FA пользователя: удаляет второй
// фактор, отзывает все незавершённые операции пользователя (их цепочки подтверждения
// построены при включённой 2FA) и отправляет уведомление.
func (uc *Disable2FA) Execute(ctx context.Context, actor dto.ActorMeta, payload []byte) error {
	if actor.VisitorID == uuid.Nil {
		return errors.ErrInternalIncorrectInputData.WithDetails("userId is empty")
	}

	payloadDTO, err := unit.ParseDisable2FAPayload(payload)
	if err != nil {
		return err
	}

	return uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err := uc.storage.Delete(ctx, actor.VisitorID); err != nil {
			// отсутствие записи 2FA - не ошибка: применение операции идемпотентно.
			// Достижимо, когда операция отключения открыта повторно (2FA уже выключена
			// предыдущей) либо когда один и тот же токен применяется конкурентно.
			// Уведомление при этом не отправляется: событие уже произошло при первом
			// применении, и письмо о нём тогда же и ушло, а повтор был бы дубликатом
			if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
				return nil
			}

			return uc.errorWrapper.Wrap(err)
		}

		// 2FA отключена именно этим применением, поэтому незавершённые операции пользователя,
		// построенные при включённой 2FA, отзываются. Если 2FA уже была снята (ветка выше),
		// сюда не доходим: операции, открытые после снятия, построены без неё и остаются в силе
		if err := uc.revoker.RevokeAll(ctx, actor, logreason.Auth2FAStateChanged); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if err := uc.notifierAPI.Send(ctx, "user.2fa.disabled", conv.Group{"to": payloadDTO.Email}); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		return nil
	})
}
