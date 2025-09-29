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
	// ChangeEmail - компонент для извлечения настроек, которые хранятся в хранилище данных.
	ChangeEmail struct {
		txManager    mrstorage.DBTxManager
		storage      mrauth.UserStorage
		notifierAPI  mrnotifier.NoticeProducer
		errorWrapper core.UseCaseErrorWrapper
	}
)

// NewChangeEmail - создаёт объект ChangeEmail.
func NewChangeEmail(
	txManager mrstorage.DBTxManager,
	storage mrauth.UserStorage,
	notifierAPI mrnotifier.NoticeProducer,
) *ChangeEmail {
	return &ChangeEmail{
		txManager:    txManager,
		storage:      storage,
		notifierAPI:  notifierAPI,
		errorWrapper: core.NewUseCaseErrorWrapper(entity.ModelNameUser),
	}
}

// Execute - возвращает строковое значение настройки с указанным идентификатором.
func (uc *ChangeEmail) Execute(ctx context.Context, userID uuid.UUID, payload []byte) error {
	payloadDTO := dto.ChangeEmailOperation{}

	if err := json.Unmarshal(payload, &payloadDTO); err != nil {
		return mr.ErrUseCaseIncorrectInternalInputData.Wrap(err, "payload", payload)
	}

	return uc.txManager.Do(ctx, func(ctx context.Context) error {
		if err := uc.storage.UpdateEmail(ctx, userID, payloadDTO.NewEmail); err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		if err := uc.notifierAPI.SendNotice(ctx, "user.email.changed", mrargs.Group{"to": payloadDTO.NotifyByEmail}); err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		return nil
	})
}
