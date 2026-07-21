package handler

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// Disable2FA - обработчик отключения 2FA пользователя.
	Disable2FA struct {
		txManager    mrstorage.DBTxManager
		storage      user2faDisabler
		notifierAPI  mrnotifier.NoteProducer
		errorWrapper errors.Wrapper
	}

	user2faDisabler interface {
		Delete(ctx context.Context, userID uuid.UUID) error
	}
)

// NewDisable2FA - создаёт объект Disable2FA.
func NewDisable2FA(
	txManager mrstorage.DBTxManager,
	storage user2faDisabler,
	notifierAPI mrnotifier.NoteProducer,
) *Disable2FA {
	return &Disable2FA{
		txManager:    txManager,
		storage:      storage,
		notifierAPI:  notifierAPI,
		errorWrapper: errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - применяет подтверждённую операцию отключения 2FA пользователя.
func (uc *Disable2FA) Execute(ctx context.Context, userID uuid.UUID, payload []byte) error {
	payloadDTO, err := unit.ParseDisable2FAPayload(payload)
	if err != nil {
		return err
	}

	return uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err := uc.storage.Delete(ctx, userID); err != nil {
			// отсутствие записи 2FA - не ошибка: подтверждение операции идемпотентно,
			// и повторное применение застаёт 2FA уже отключённой. Уведомление при этом
			// не отправляется: событие уже произошло при первом применении, и письмо
			// о нём тогда же и ушло, а повтор был бы дубликатом об одном и том же
			if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
				return nil
			}

			return uc.errorWrapper.Wrap(err)
		}

		if err := uc.notifierAPI.Send(ctx, "user.2fa.disabled", conv.Group{"to": payloadDTO.Email}); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		return nil
	})
}
