package handler

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrargs"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ChangePhone - comment struct.
	ChangePhone struct {
		txManager    mrstorage.DBTxManager
		storage      mrauth.UserStorage
		notifierAPI  mrnotifier.NoticeProducer
		errorWrapper mrerr.UseCaseErrorWrapper
	}
)

// NewChangePhone - создаёт объект ChangePhone.
func NewChangePhone(
	txManager mrstorage.DBTxManager,
	storage mrauth.UserStorage,
	notifierAPI mrnotifier.NoticeProducer,
	errorWrapper mrerr.UseCaseErrorWrapper,
) *ChangePhone {
	return &ChangePhone{
		txManager:    txManager,
		storage:      storage,
		notifierAPI:  notifierAPI,
		errorWrapper: mrerr.NewUseCaseErrorWrapper(errorWrapper, entity.ModelNameUser),
	}
}

// Execute - возвращает строковое значение настройки с указанным идентификатором.
func (uc *ChangePhone) Execute(ctx context.Context, userID uuid.UUID, payload []byte) error {
	payloadDTO := dto.ChangePhoneOperation{}

	if err := json.Unmarshal(payload, &payloadDTO); err != nil {
		return mr.ErrUseCaseIncorrectInternalInputData.Wrap(err, "payload", payload)
	}

	return uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err := uc.storage.UpdatePhone(ctx, userID, payloadDTO.NewPhone); err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		if err := uc.notifierAPI.SendNotice(ctx, "user.phone.changed", mrargs.Group{"to": payloadDTO.NotifyByEmail}); err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		return nil
	})
}
