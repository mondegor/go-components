package auth

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"

	core "github.com/mondegor/go-components/internal"
	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// UserInfo - компонент для извлечения настроек, которые хранятся в хранилище данных.
	UserInfo struct {
		txManager        mrstorage.DBTxManager
		storageUser      mrauth.UserStorage
		storageUser2FA   mrauth.User2faStorage
		storageUserStat  mrauth.UserActivityStatStorage
		storageUserRealm mrauth.UserRealmStorage
		errorWrapper     core.UseCaseErrorWrapper
	}
)

// NewUserInfo - создаёт объект UserInfo.
func NewUserInfo(
	txManager mrstorage.DBTxManager,
	storageUser mrauth.UserStorage,
	storageUser2FA mrauth.User2faStorage,
	storageUserStat mrauth.UserActivityStatStorage,
	storageUserRealm mrauth.UserRealmStorage,
) *UserInfo {
	return &UserInfo{
		txManager:        txManager,
		storageUser:      storageUser,
		storageUser2FA:   storageUser2FA,
		storageUserStat:  storageUserStat,
		storageUserRealm: storageUserRealm,
		errorWrapper:     core.NewUseCaseErrorWrapper(entity.ModelNameUser),
	}
}

// Get - возвращает строковое значение настройки с указанным идентификатором.
func (uc *UserInfo) Get(ctx context.Context, userID uuid.UUID) (userInfo dto.UserInfo, err error) {
	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		if userInfo.User, err = uc.storageUser.FetchOne(ctx, userID); err != nil {
			return uc.errorWrapper.WrapErrorFailed(err) // the user must be
		}

		if userInfo.Auth2fa, err = uc.storageUser2FA.FetchOne(ctx, userID); err != nil {
			if !uc.errorWrapper.IsNotFoundOrNotAffectedError(err) {
				return uc.errorWrapper.WrapErrorFailed(err)
			}
		}

		if userInfo.Stat, err = uc.storageUserStat.FetchOne(ctx, userID); err != nil {
			if !uc.errorWrapper.IsNotFoundOrNotAffectedError(err) {
				return uc.errorWrapper.WrapErrorFailed(err)
			}
		}

		if userInfo.Realms, err = uc.storageUserRealm.Fetch(ctx, userID); err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		return nil
	})
	if err != nil {
		return dto.UserInfo{}, err
	}

	return userInfo, nil
}
