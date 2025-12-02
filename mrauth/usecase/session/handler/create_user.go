package handler

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrargs"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"
	"github.com/mondegor/go-sysmess/mrlog"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/bag/contactaddress"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/userstatus"
	"github.com/mondegor/go-components/mrnotifier"
)

type (
	// CreateUser - компонент для извлечения настроек, которые хранятся в хранилище данных.
	CreateUser struct {
		txManager        mrstorage.DBTxManager
		storageUser      mrauth.UserStorage
		storageUserRealm mrauth.UserRealmStorage
		notifierAPI      mrnotifier.NoticeProducer
		errorWrapper     mrerr.UseCaseErrorWrapper
		logger           mrlog.Logger
	}
)

// NewCreateUser - создаёт объект CreateUser.
func NewCreateUser(
	txManager mrstorage.DBTxManager,
	storageUser mrauth.UserStorage,
	storageUserNamespace mrauth.UserRealmStorage,
	notifierAPI mrnotifier.NoticeProducer,
	errorWrapper mrerr.UseCaseErrorWrapper,
	logger mrlog.Logger,
) *CreateUser {
	return &CreateUser{
		txManager:        txManager,
		storageUser:      storageUser,
		storageUserRealm: storageUserNamespace,
		notifierAPI:      notifierAPI,
		errorWrapper:     mrerr.NewUseCaseErrorWrapper(errorWrapper, entity.ModelNameUser),
		logger:           logger,
	}
}

// Execute - возвращает строковое значение настройки с указанным идентификатором.
func (uc *CreateUser) Execute(ctx context.Context, payload []byte) (u dto.UserInRealm, err error) {
	payloadDTO := dto.CreateUserOperation{}

	if err = json.Unmarshal(payload, &payloadDTO); err != nil {
		return dto.UserInRealm{}, mr.ErrUseCaseIncorrectInternalInputData.Wrap(err, "payload", payload)
	}

	user, err := uc.storageUser.FetchOneByLogin(ctx, contactaddress.NewEmail(payloadDTO.Email))
	if err != nil {
		if !mr.ErrStorageNoRowFound.Is(err) {
			return dto.UserInRealm{}, uc.errorWrapper.WrapErrorFailed(err)
		}
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		if user.ID == uuid.Nil {
			user = entity.User{
				Email:    payloadDTO.Email,
				LangCode: payloadDTO.LangCode,
				Status:   userstatus.Enabled,
			}

			user.ID, err = uc.storageUser.Insert(ctx, user)
			if err != nil {
				return uc.errorWrapper.WrapErrorFailed(err)
			}
		}

		userRealm := entity.UserRealm{
			UserID: user.ID,
			Realm:  payloadDTO.Realm,
			Kind:   payloadDTO.UserKind,
		}

		if err = uc.storageUserRealm.Insert(ctx, userRealm); err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		return nil
	})
	if err != nil {
		return dto.UserInRealm{}, err
	}

	err = uc.notifierAPI.SendNotice(
		ctx,
		"user.registration.success",
		mrargs.Group{
			"lang": payloadDTO.LangCode,
			"to":   payloadDTO.Email,
		},
	)
	if err != nil {
		uc.logger.Error(ctx, "After CreateUser notice 'user.registration.success' not send", "error", err)
	}

	err = uc.notifierAPI.SendNotice(
		ctx,
		"user.was.registered",
		mrargs.Group{
			"lang":      payloadDTO.LangCode,
			"userRealm": payloadDTO.Realm,
			"userEmail": payloadDTO.Email,
		},
	)
	if err != nil {
		uc.logger.Error(ctx, "After CreateUser notice 'user.was.registered' not send", "error", err)
	}

	return dto.UserInRealm{
		ID:       user.ID,
		Realm:    payloadDTO.Realm,
		Kind:     payloadDTO.UserKind,
		LangCode: user.LangCode,
		// Email:    user.Email,
		// Phone:    user.Phone,
	}, nil
}
