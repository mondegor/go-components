package handler

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrargs"
	"github.com/mondegor/go-sysmess/mrerr/mr"

	core "github.com/mondegor/go-components/internal"
	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// Disable2FA - компонент для извлечения настроек, которые хранятся в хранилище данных.
	Disable2FA struct {
		txManager    mrstorage.DBTxManager
		storage      mrauth.User2faStorage
		notifierAPI  mrnotifier.NoticeProducer
		errorWrapper core.UseCaseErrorWrapper
	}
)

// NewDisable2FA - создаёт объект ChangePhone.
func NewDisable2FA(
	txManager mrstorage.DBTxManager,
	storage mrauth.User2faStorage,
	notifierAPI mrnotifier.NoticeProducer,
) *Disable2FA {
	return &Disable2FA{
		txManager:    txManager,
		storage:      storage,
		notifierAPI:  notifierAPI,
		errorWrapper: core.NewUseCaseErrorWrapper(entity.ModelNameUser),
	}
}

// Execute - возвращает строковое значение настройки с указанным идентификатором.
func (uc *Disable2FA) Execute(ctx context.Context, userID uuid.UUID, payload []byte) error {
	payloadDTO := dto.Disable2faOperation{}

	if err := json.Unmarshal(payload, &payloadDTO); err != nil {
		return mr.ErrUseCaseIncorrectInternalInputData.Wrap(err, "payload", payload)
	}

	return uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err := uc.storage.Delete(ctx, userID); err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		if err := uc.notifierAPI.SendNotice(ctx, "user.2fa.disabled", mrargs.Group{"to": payloadDTO.Email}); err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		return nil
	})
}
