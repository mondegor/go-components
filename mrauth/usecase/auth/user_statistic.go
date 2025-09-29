package auth

import (
	"bytes"
	"context"
	"slices"

	"github.com/mondegor/go-sysmess/mrerr/mr"

	core "github.com/mondegor/go-components/internal"
	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// UserStatistic - компонент для извлечения настроек, которые хранятся в хранилище данных.
	UserStatistic struct {
		storageActivityStat mrauth.UserActivityStatStorage
		storageActivityLog  mrauth.UserActivityLogStorage
		errorWrapper        core.UseCaseErrorWrapper
	}
)

// NewUserStatistic - создаёт объект Session.
func NewUserStatistic(
	storageActivityStat mrauth.UserActivityStatStorage,
	storageActivityLog mrauth.UserActivityLogStorage,
) *UserStatistic {
	return &UserStatistic{
		storageActivityStat: storageActivityStat,
		storageActivityLog:  storageActivityLog,
		errorWrapper:        core.NewUseCaseErrorWrapper(entity.ModelNameRefreshToken),
	}
}

// Execute - comments method.
func (uc *UserStatistic) Execute(ctx context.Context, list []dto.UserActivityLog) error {
	if len(list) == 0 {
		return nil
	}

	usersN := 1

	slices.SortFunc(list, func(a, b dto.UserActivityLog) int {
		if cmp := bytes.Compare(a.UserID[:], b.UserID[:]); cmp != 0 {
			usersN++ // подсчёт уникальных пользователей

			return cmp
		}

		return b.VisitedAt.Compare(a.VisitedAt) // сортировка по времени в обратном порядке
	})

	stat := make([]entity.UserActivityLastVisited, usersN)

	stat[0] = entity.UserActivityLastVisited{
		UserID:        list[0].UserID,
		LastVisitedAt: list[0].VisitedAt,
	}

	j := 0

	for i := 1; i < len(list); i++ {
		if list[i].UserID == stat[j].UserID {
			continue
		}

		j++

		stat[j] = entity.UserActivityLastVisited{
			UserID:        list[i].UserID,
			LastVisitedAt: list[i].VisitedAt,
		}
	}

	// целостность данных не критична, поэтому единой транзакции не требуется

	if err := uc.storageActivityStat.UpdateLastVisited(ctx, stat); err != nil {
		if !mr.ErrStorageRowsNotAffected.Is(err) { // TODO: а может быть ситуация, когда данные не обновились?
			return uc.errorWrapper.WrapErrorFailed(err)
		}
	}

	if err := uc.storageActivityLog.Insert(ctx, list); err != nil {
		return uc.errorWrapper.WrapErrorFailed(err)
	}

	return nil
}
