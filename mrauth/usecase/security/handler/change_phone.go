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
	// ChangePhone - comment struct.
	ChangePhone struct {
		txManager    mrstorage.DBTxManager
		storage      userPhoneChanger
		notifierAPI  mrnotifier.NoteProducer
		errorWrapper errors.Wrapper
	}

	userPhoneChanger interface {
		UpdatePhone(ctx context.Context, userID uuid.UUID, value uint64) error
	}
)

// NewChangePhone - создаёт объект ChangePhone.
func NewChangePhone(
	txManager mrstorage.DBTxManager,
	storage userPhoneChanger,
	notifierAPI mrnotifier.NoteProducer,
) *ChangePhone {
	return &ChangePhone{
		txManager:    txManager,
		storage:      storage,
		notifierAPI:  notifierAPI,
		errorWrapper: errors.NewUseCaseWrapper(),
	}
}

// Execute - возвращает строковое значение настройки с указанным идентификатором.
func (uc *ChangePhone) Execute(ctx context.Context, userID uuid.UUID, payload []byte) error {
	payloadDTO := dto.ChangePhoneOperation{}

	if err := json.Unmarshal(payload, &payloadDTO); err != nil {
		return errors.ErrInternalIncorrectInputData.WithError(err, "ChangePhone", "payload", payload)
	}

	return uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err := uc.storage.UpdatePhone(ctx, userID, payloadDTO.NewPhone); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if err := uc.notifierAPI.Send(ctx, "user.phone.changed", conv.Group{"to": payloadDTO.NotifyByEmail}); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		return nil
	})
}
