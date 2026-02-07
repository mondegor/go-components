package userinfo

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// UserInfo - comment struct.
	UserInfo struct {
		txManager        mrstorage.DBTxManager
		storageUser      userFetcher
		storageUser2FA   user2faFetcher
		storageUserStat  userActivityStatFetcher
		storageUserRealm userRealmFetcher
		errorWrapper     errors.Wrapper
	}

	userFetcher interface {
		FetchOne(ctx context.Context, userID uuid.UUID) (entity.User, error)
	}

	user2faFetcher interface {
		FetchOne(ctx context.Context, userID uuid.UUID) (row entity.Auth2fa, err error)
	}

	userActivityStatFetcher interface {
		FetchOne(ctx context.Context, userID uuid.UUID) (row entity.UserActivityStat, err error)
	}

	userRealmFetcher interface {
		Fetch(ctx context.Context, userID uuid.UUID) ([]entity.UserRealm, error)
	}
)

// New - создаёт объект UserInfo.
func New(
	txManager mrstorage.DBTxManager,
	storageUser userFetcher,
	storageUser2FA user2faFetcher,
	storageUserStat userActivityStatFetcher,
	storageUserRealm userRealmFetcher,
) *UserInfo {
	return &UserInfo{
		txManager:        txManager,
		storageUser:      storageUser,
		storageUser2FA:   storageUser2FA,
		storageUserStat:  storageUserStat,
		storageUserRealm: storageUserRealm,
		errorWrapper:     errors.NewInfraStorageWrapper(),
	}
}

// Get - возвращает строковое значение настройки с указанным идентификатором.
func (sv *UserInfo) Get(ctx context.Context, userID uuid.UUID) (userInfo dto.UserInfo, err error) {
	err = sv.txManager.Do(ctx, func(ctx context.Context) error {
		if userInfo.User, err = sv.storageUser.FetchOne(ctx, userID); err != nil {
			return sv.errorWrapper.Wrap(err) // the user must be
		}

		if userInfo.Auth2fa, err = sv.storageUser2FA.FetchOne(ctx, userID); err != nil {
			if !errors.Is(err, errors.ErrEventStorageNoRowFound) {
				return sv.errorWrapper.Wrap(err)
			}
		}

		if userInfo.Stat, err = sv.storageUserStat.FetchOne(ctx, userID); err != nil {
			if !errors.Is(err, errors.ErrEventStorageNoRowFound) {
				return sv.errorWrapper.Wrap(err)
			}
		}

		if userInfo.Realms, err = sv.storageUserRealm.Fetch(ctx, userID); err != nil {
			return sv.errorWrapper.Wrap(err)
		}

		return nil
	})
	if err != nil {
		return dto.UserInfo{}, err
	}

	return userInfo, nil
}
