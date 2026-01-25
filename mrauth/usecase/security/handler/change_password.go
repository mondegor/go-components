package handler

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// ChangePassword - comment struct.
	ChangePassword struct {
		storage      mrauth.User2faStorage
		notifierAPI  mrnotifier.NoteProducer
		errorWrapper errors.Wrapper
		logger       mrlog.Logger
	}
)

// NewChangePassword - создаёт объект ChangePassword.
func NewChangePassword(
	storage mrauth.User2faStorage,
	notifierAPI mrnotifier.NoteProducer,
	logger mrlog.Logger,
) *ChangePassword {
	return &ChangePassword{
		storage:      storage,
		notifierAPI:  notifierAPI,
		errorWrapper: errors.NewUseCaseWrapper(),
		logger:       logger,
	}
}

// Execute - возвращает строковое значение настройки с указанным идентификатором.
func (uc *ChangePassword) Execute(ctx context.Context, userID uuid.UUID, payload []byte) error {
	payloadDTO := dto.ChangePasswordOperation{}

	if err := json.Unmarshal(payload, &payloadDTO); err != nil {
		return errors.ErrInternalIncorrectInputData.WithError(err, "ChangePassword", "payload", payload)
	}

	err := uc.storage.InsertOrUpdate(
		ctx,
		entity.Auth2fa{
			UserID: userID,
			Type:   auth2fatype.Password,
			Secret: payloadDTO.NewPassword,
		},
	)
	if err != nil {
		return uc.errorWrapper.Wrap(err)
	}

	// TODO: если важно, чтобы пользователь получил сообщение, то нужно завернуть в транзакцию
	if err := uc.notifierAPI.Send(ctx, "user.password.changed", conv.Group{"to": payloadDTO.NotifyByEmail}); err != nil {
		uc.logger.Error(ctx, "After ChangePassword message not send", "error", err)
	}

	return nil
}
