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
	// ChangePhone - обработчик смены телефона пользователя.
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
		errorWrapper: errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - применяет подтверждённую операцию смены телефона пользователя.
func (uc *ChangePhone) Execute(ctx context.Context, userID uuid.UUID, payload []byte) error {
	payloadDTO, err := unit.ParseChangePhonePayload(payload)
	if err != nil {
		return err
	}

	return uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err := uc.storage.UpdatePhone(ctx, userID, payloadDTO.NewPhone); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		if err := uc.notifierAPI.Send(ctx, "user.phone.changed", conv.Group{"to": payloadDTO.Email}); err != nil {
			return uc.errorWrapper.Wrap(err)
		}

		return nil
	})
}
