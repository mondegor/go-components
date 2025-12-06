package auth

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrerr"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// UserInfo - comment struct.
	UserInfo struct {
		txManager        mrstorage.DBTxManager
		storageUser      mrauth.UserStorage
		storageUser2FA   mrauth.User2faStorage
		storageUserStat  mrauth.UserActivityStatStorage
		storageUserRealm mrauth.UserRealmStorage
		errorWrapper     mrerr.UseCaseErrorWrapper
	}
)

// NewUserInfo - создаёт объект UserInfo.
func NewUserInfo(
	txManager mrstorage.DBTxManager,
	storageUser mrauth.UserStorage,
	storageUser2FA mrauth.User2faStorage,
	storageUserStat mrauth.UserActivityStatStorage,
	storageUserRealm mrauth.UserRealmStorage,
	errorWrapper mrerr.UseCaseErrorWrapper,
) *UserInfo {
	return &UserInfo{
		txManager:        txManager,
		storageUser:      storageUser,
		storageUser2FA:   storageUser2FA,
		storageUserStat:  storageUserStat,
		storageUserRealm: storageUserRealm,
		errorWrapper:     mrerr.NewUseCaseErrorWrapper(errorWrapper, "mrauth.UserInfo"),
	}
}

// Execute - возвращает строковое значение настройки с указанным идентификатором.
func (uc *UserInfo) Execute(ctx context.Context, userID uuid.UUID) (userInfo entity.UserInfo, err error) {
	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		if userInfo.User, err = uc.storageUser.FetchOne(ctx, userID); err != nil {
			return uc.errorWrapper.WrapErrorFailed(err) // the user must be
		}

		if userInfo.Auth2fa, err = uc.storageUser2FA.FetchOne(ctx, userID); err != nil {
			if !uc.errorWrapper.IsNotFoundError(err) {
				return uc.errorWrapper.WrapErrorFailed(err)
			}
		}

		if userInfo.Stat, err = uc.storageUserStat.FetchOne(ctx, userID); err != nil {
			if !uc.errorWrapper.IsNotFoundError(err) {
				return uc.errorWrapper.WrapErrorFailed(err)
			}
		}

		if userInfo.Realms, err = uc.storageUserRealm.Fetch(ctx, userID); err != nil {
			return uc.errorWrapper.WrapErrorFailed(err)
		}

		return nil
	})
	if err != nil {
		return entity.UserInfo{}, err
	}

	return userInfo, nil
}
