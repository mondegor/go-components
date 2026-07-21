package userinfo

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// UserInfo - сервис получения сводной информации о пользователе.
	UserInfo struct {
		txManager        mrstorage.DBTxManager
		storageUser      userFetcher
		storageUser2FA   user2faFetcher
		storageUserStat  userActivityStatFetcher
		storageUserRealm userRealmFetcher
		locationResolver mrauth.LocationResolver
		errorWrapper     errors.Wrapper
	}

	userFetcher interface {
		FetchOne(ctx context.Context, userID uuid.UUID) (entity.User, error)
	}

	user2faFetcher interface {
		FetchOne(ctx context.Context, userID uuid.UUID) (row entity.Auth2FA, err error)
	}

	userActivityStatFetcher interface {
		Fetch(ctx context.Context, userID uuid.UUID) ([]entity.UserActivityStat, error)
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
	locationResolver mrauth.LocationResolver,
) *UserInfo {
	if locationResolver == nil {
		locationResolver = mrauth.DefaultLocationResolver
	}

	return &UserInfo{
		txManager:        txManager,
		storageUser:      storageUser,
		storageUser2FA:   storageUser2FA,
		storageUserStat:  storageUserStat,
		storageUserRealm: storageUserRealm,
		locationResolver: locationResolver,
		errorWrapper:     errors.NewServiceOperationFailedWrapper(),
	}
}

// Get - возвращает сводную информацию о пользователе вместе со статистикой входа по каждому realm'у.
func (sv *UserInfo) Get(ctx context.Context, userID uuid.UUID) (dto.UserInfo, error) {
	var (
		user    entity.User
		auth2FA entity.Auth2FA
		stats   []entity.UserActivityStat
		realms  []entity.UserRealm
		err     error
	)

	err = sv.txManager.Do(ctx, func(ctx context.Context) error {
		if user, err = sv.storageUser.FetchOne(ctx, userID); err != nil {
			return sv.errorWrapper.Wrap(err) // the user must be
		}

		if auth2FA, err = sv.storageUser2FA.FetchOne(ctx, userID); err != nil {
			if !errors.Is(err, errors.ErrEventStorageNoRecordFound) {
				return sv.errorWrapper.Wrap(err)
			}
		}

		if stats, err = sv.storageUserStat.Fetch(ctx, userID); err != nil {
			return sv.errorWrapper.Wrap(err)
		}

		if realms, err = sv.storageUserRealm.Fetch(ctx, userID); err != nil {
			return sv.errorWrapper.Wrap(err)
		}

		return nil
	})
	if err != nil {
		return dto.UserInfo{}, err
	}

	return dto.UserInfo{
		User:    user,
		Auth2FA: auth2FA,
		Realms:  sv.buildRealms(realms, stats),
	}, nil
}

// buildRealms - объединяет привязки к realm'ам со статистикой последнего входа по каждому из них.
// Для realm'а без записи статистики местоположение остаётся пустым, а время входа - нулевым.
func (sv *UserInfo) buildRealms(realms []entity.UserRealm, stats []entity.UserActivityStat) []dto.UserRealmInfo {
	statByRealm := make(map[uint16]entity.UserActivityStat, len(stats))

	for _, stat := range stats {
		statByRealm[stat.RealmID] = stat
	}

	list := make([]dto.UserRealmInfo, 0, len(realms))

	for _, realm := range realms {
		item := dto.UserRealmInfo{
			RealmID:   realm.RealmID,
			Kind:      realm.Kind,
			CreatedAt: realm.CreatedAt,
			UpdatedAt: realm.UpdatedAt,
		}

		// запись заводится только при входе, а last_login_ip/last_logged_at объявлены NOT NULL
		// (см. _sample/migrations, users_activity_stat), поэтому в найденной строке IP и дата входа есть
		if stat, ok := statByRealm[realm.RealmID]; ok {
			item.LastLocation = sv.locationResolver(stat.LastLoginIP, mrauth.LocationOrIP)
			item.LastLoggedAt = stat.LastLoggedAt
		}

		list = append(list, item)
	}

	return list
}
