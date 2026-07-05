package handler

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ChangeEmail - обработчик смены email пользователя.
	ChangeEmail struct {
		txManager    mrstorage.DBTxManager
		storage      userEmailChanger
		notifierAPI  mrnotifier.NoteProducer
		errorWrapper errors.Wrapper
	}

	userEmailChanger interface {
		UpdateEmail(ctx context.Context, userID uuid.UUID, value string) error
	}
)

// NewChangeEmail - создаёт объект ChangeEmail.
func NewChangeEmail(
	txManager mrstorage.DBTxManager,
	storage userEmailChanger,
	notifierAPI mrnotifier.NoteProducer,
) *ChangeEmail {
	return &ChangeEmail{
		txManager:    txManager,
		storage:      storage,
		notifierAPI:  notifierAPI,
		errorWrapper: errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - возвращает строковое значение настройки с указанным идентификатором.
func (uc *ChangeEmail) Execute(ctx context.Context, userID uuid.UUID, payload []byte) error {
	payloadDTO := dto.ChangeEmailOperation{}

	if err := json.Unmarshal(payload, &payloadDTO); err != nil {
		return errors.ErrInternalIncorrectInputData.WithError(err, "ChangeEmail", "payload", payload)
	}

	return uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err := uc.storage.UpdateEmail(ctx, userID, payloadDTO.NewEmail); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if err := uc.notifierAPI.Send(ctx, "user.email.changed", conv.Group{"to": payloadDTO.Email}); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		return nil
	})
}
