package handler

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/mrargs"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"
	"github.com/mondegor/go-sysmess/mrlog"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ChangePassword - компонент для извлечения настроек, которые хранятся в хранилище данных.
	ChangePassword struct {
		storage      mrauth.User2faStorage
		notifierAPI  mrnotifier.NoticeProducer
		errorWrapper mrerr.UseCaseErrorWrapper
		logger       mrlog.Logger
	}
)

// NewChangePassword - создаёт объект ChangePassword.
func NewChangePassword(
	storage mrauth.User2faStorage,
	notifierAPI mrnotifier.NoticeProducer,
	errorWrapper mrerr.UseCaseErrorWrapper,
	logger mrlog.Logger,
) *ChangePassword {
	return &ChangePassword{
		storage:      storage,
		notifierAPI:  notifierAPI,
		errorWrapper: mrerr.NewUseCaseErrorWrapper(errorWrapper, entity.ModelNameAuth2fa),
		logger:       logger,
	}
}

// Execute - возвращает строковое значение настройки с указанным идентификатором.
func (uc *ChangePassword) Execute(ctx context.Context, userID uuid.UUID, payload []byte) error {
	payloadDTO := dto.ChangePasswordOperation{}

	if err := json.Unmarshal(payload, &payloadDTO); err != nil {
		return mr.ErrUseCaseIncorrectInternalInputData.Wrap(err, "payload", payload)
	}

	err := uc.storage.InsertOrUpdate(
		ctx,
		entity.Auth2fa{
			UserID: userID,
			Type:   enum.Auth2faTypePassword,
			Secret: payloadDTO.NewPassword,
		},
	)
	if err != nil {
		return uc.errorWrapper.WrapErrorFailed(err)
	}

	// TODO: если важно, чтобы пользователь получил сообщение, то нужно завернуть в транзакцию
	if err := uc.notifierAPI.SendNotice(ctx, "user.password.changed", mrargs.Group{"to": payloadDTO.NotifyByEmail}); err != nil {
		uc.logger.Error(ctx, "After ChangePassword message not send", "error", err)
	}

	return nil
}
