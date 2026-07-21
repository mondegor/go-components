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
		errorWrapper: errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - применяет подтверждённую операцию смены email пользователя.
func (uc *ChangeEmail) Execute(ctx context.Context, userID uuid.UUID, payload []byte) error {
	payloadDTO, err := unit.ParseChangeEmailPayload(payload)
	if err != nil {
		return err
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
