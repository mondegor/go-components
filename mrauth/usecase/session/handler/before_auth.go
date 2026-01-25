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
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// BeforeAuthUser - comment struct.
	BeforeAuthUser struct {
		storageUser      mrauth.UserStorage
		storageUserRealm mrauth.UserRealmStorage
		notifierAPI      mrnotifier.NoteProducer
		errorWrapper     errors.Wrapper
		logger           mrlog.Logger
	}
)

// NewBeforeAuthUser - создаёт объект BeforeAuthUser.
func NewBeforeAuthUser(
	storageUser mrauth.UserStorage,
	storageUserNamespace mrauth.UserRealmStorage,
	notifierAPI mrnotifier.NoteProducer,
	logger mrlog.Logger,
) *BeforeAuthUser {
	return &BeforeAuthUser{
		storageUser:      storageUser,
		storageUserRealm: storageUserNamespace,
		notifierAPI:      notifierAPI,
		errorWrapper:     errors.NewUseCaseWrapper(),
		logger:           logger,
	}
}

// Execute - возвращает строковое значение настройки с указанным идентификатором.
func (uc *BeforeAuthUser) Execute(ctx context.Context, userID uuid.UUID, payload []byte) (u dto.UserInRealm, err error) {
	if userID == uuid.Nil {
		return dto.UserInRealm{}, errors.ErrInternalIncorrectInputData.WithDetails("userID is zero")
	}

	payloadDTO := dto.AuthorizeUserOperation{}

	if err := json.Unmarshal(payload, &payloadDTO); err != nil {
		return dto.UserInRealm{}, errors.ErrInternalIncorrectInputData.WithError(err, "BeforeAuthUser", "payload", payload)
	}

	user, err := uc.storageUser.FetchOne(ctx, userID)
	if err != nil {
		return dto.UserInRealm{}, uc.errorWrapper.Wrap(err, "userId", userID)
	}

	userRealm, err := uc.storageUserRealm.FetchOne(ctx, userID, payloadDTO.Realm)
	if err != nil {
		return dto.UserInRealm{}, uc.errorWrapper.Wrap(err, "userId", userID, "realm", payloadDTO.Realm)
	}

	// TODO: добавить логику, чтобы отправлять сообщение, если авторизация произошла на новом устройстве

	if err := uc.notifierAPI.Send(ctx, "user.authorization.success", conv.Group{"lang": payloadDTO.LangCode, "to": user.Email}); err != nil {
		uc.logger.Error(ctx, "After BeforeAuthUser notice 'user.authorization.success' not send", "error", err)
	}

	return dto.UserInRealm{
		ID:       user.ID,
		Realm:    userRealm.Realm,
		Kind:     userRealm.Kind,
		LangCode: user.LangCode,
		// Email:    user.Email,
		// Phone:    user.Phone,
	}, nil
}
