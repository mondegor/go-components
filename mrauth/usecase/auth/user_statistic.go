package auth

import (
	"bytes"
	"context"
	"slices"

	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth/dto"
)

type (
	// UserStatistic - comment struct.
	UserStatistic struct {
		storageActivityStat userActivityStatUpdater
		storageActivityLog  userActivityLogStorage
		errorWrapper        errors.Wrapper
	}

	userActivityStatUpdater interface {
		UpdateLastVisited(ctx context.Context, rows []dto.UserActivityLastVisited) error
	}

	userActivityLogStorage interface {
		Insert(ctx context.Context, rows []dto.UserActivityLogMessage) error
	}
)

// NewUserStatistic - создаёт объект Session.
func NewUserStatistic(
	storageActivityStat userActivityStatUpdater,
	storageActivityLog userActivityLogStorage,
) *UserStatistic {
	return &UserStatistic{
		storageActivityStat: storageActivityStat,
		storageActivityLog:  storageActivityLog,
		errorWrapper:        errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - comments method.
func (uc *UserStatistic) Execute(ctx context.Context, messages []dto.UserActivityLogMessage) error {
	if len(messages) == 0 {
		return nil
	}

	usersN := 1

	slices.SortFunc(messages, func(a, b dto.UserActivityLogMessage) int {
		if cmp := bytes.Compare(a.UserID[:], b.UserID[:]); cmp != 0 {
			usersN++ // подсчёт уникальных пользователей

			return cmp
		}

		return b.VisitedAt.Compare(a.VisitedAt) // сортировка по времени в обратном порядке
	})

	stat := make([]dto.UserActivityLastVisited, usersN)

	stat[0] = dto.UserActivityLastVisited{
		UserID:        messages[0].UserID,
		LastVisitedAt: messages[0].VisitedAt,
	}

	j := 0

	for i := 1; i < len(messages); i++ {
		if messages[i].UserID == stat[j].UserID {
			continue
		}

		j++

		stat[j] = dto.UserActivityLastVisited{
			UserID:        messages[i].UserID,
			LastVisitedAt: messages[i].VisitedAt,
		}
	}

	// целостность данных не критична, поэтому единой транзакции не требуется

	if err := uc.storageActivityStat.UpdateLastVisited(ctx, stat); err != nil {
		return uc.errorWrapper.Wrap(err)
	}

	if err := uc.storageActivityLog.Insert(ctx, messages); err != nil {
		return uc.errorWrapper.Wrap(err)
	}

	return nil
}
