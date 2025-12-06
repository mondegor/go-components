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
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// BeforeAuthUser - comment struct.
	BeforeAuthUser struct {
		storageUser      mrauth.UserStorage
		storageUserRealm mrauth.UserRealmStorage
		notifierAPI      mrnotifier.NoticeProducer
		errorWrapper     mrerr.UseCaseErrorWrapper
		logger           mrlog.Logger
	}
)

// NewBeforeAuthUser - создаёт объект BeforeAuthUser.
func NewBeforeAuthUser(
	storageUser mrauth.UserStorage,
	storageUserNamespace mrauth.UserRealmStorage,
	notifierAPI mrnotifier.NoticeProducer,
	errorWrapper mrerr.UseCaseErrorWrapper,
	logger mrlog.Logger,
) *BeforeAuthUser {
	return &BeforeAuthUser{
		storageUser:      storageUser,
		storageUserRealm: storageUserNamespace,
		notifierAPI:      notifierAPI,
		errorWrapper:     mrerr.NewUseCaseErrorWrapper(errorWrapper, "mrauth.BeforeAuthUser"),
		logger:           logger,
	}
}

// Execute - возвращает строковое значение настройки с указанным идентификатором.
func (uc *BeforeAuthUser) Execute(ctx context.Context, userID uuid.UUID, payload []byte) (u dto.UserInRealm, err error) {
	if userID == uuid.Nil {
		return dto.UserInRealm{}, mr.ErrUseCaseIncorrectInternalInputData.New("reason", "userID is zero")
	}

	payloadDTO := dto.AuthorizeUserOperation{}

	if err := json.Unmarshal(payload, &payloadDTO); err != nil {
		return dto.UserInRealm{}, mr.ErrUseCaseIncorrectInternalInputData.Wrap(err, "payload", payload)
	}

	user, err := uc.storageUser.FetchOne(ctx, userID)
	if err != nil {
		return dto.UserInRealm{}, uc.errorWrapper.WrapErrorFailed(err, "userId", userID)
	}

	userRealm, err := uc.storageUserRealm.FetchOne(ctx, userID, payloadDTO.Realm)
	if err != nil {
		return dto.UserInRealm{}, uc.errorWrapper.WrapErrorFailed(err, "userId", userID, "realm", payloadDTO.Realm)
	}

	// TODO: добавить логику, чтобы отправлять сообщение, если авторизация произошла на новом устройстве

	if err := uc.notifierAPI.SendNotice(ctx, "user.authorization.success", mrargs.Group{"lang": payloadDTO.LangCode, "to": user.Email}); err != nil {
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
