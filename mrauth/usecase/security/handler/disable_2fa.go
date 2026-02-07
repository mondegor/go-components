package handler

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/util/conv"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// Disable2FA - comment struct.
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

// NewDisable2FA - создаёт объект ChangePhone.
func NewDisable2FA(
	txManager mrstorage.DBTxManager,
	storage user2faDisabler,
	notifierAPI mrnotifier.NoteProducer,
) *Disable2FA {
	return &Disable2FA{
		txManager:    txManager,
		storage:      storage,
		notifierAPI:  notifierAPI,
		errorWrapper: errors.NewUseCaseWrapper(),
	}
}

// Execute - возвращает строковое значение настройки с указанным идентификатором.
func (uc *Disable2FA) Execute(ctx context.Context, userID uuid.UUID, payload []byte) error {
	payloadDTO := dto.Disable2faOperation{}

	if err := json.Unmarshal(payload, &payloadDTO); err != nil {
		return errors.ErrInternalIncorrectInputData.WithError(err, "Disable2FA", "payload", payload)
	}

	return uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err := uc.storage.Delete(ctx, userID); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if err := uc.notifierAPI.Send(ctx, "user.2fa.disabled", conv.Group{"to": payloadDTO.Email}); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		return nil
	})
}
