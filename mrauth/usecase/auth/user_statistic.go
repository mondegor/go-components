package auth

import (
	"bytes"
	"context"
	"slices"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth/dto"
)

type (
	// UserStatistic - обновляет статистику активности пользователей пакетом сообщений:
	// последнее посещение, активность по сессиям и журнал активности.
	UserStatistic struct {
		storageActivityStat userActivityStatUpdater
		storageActivityLog  userActivityLogStorage
		storageSession      sessionLastActivityUpdater
		errorWrapper        errors.Wrapper
	}

	userActivityStatUpdater interface {
		UpdateLastVisited(ctx context.Context, rows []dto.UserActivityLastVisited) error
	}

	userActivityLogStorage interface {
		Insert(ctx context.Context, rows []dto.UserActivityLogMessage) error
	}

	sessionLastActivityUpdater interface {
		UpdateLastActivity(ctx context.Context, rows []dto.SessionLastActivity) error
	}
)

// NewUserStatistic - создаёт объект UserStatistic.
func NewUserStatistic(
	storageActivityStat userActivityStatUpdater,
	storageActivityLog userActivityLogStorage,
	storageSession sessionLastActivityUpdater,
) *UserStatistic {
	return &UserStatistic{
		storageActivityStat: storageActivityStat,
		storageActivityLog:  storageActivityLog,
		storageSession:      storageSession,
		errorWrapper:        errors.NewServiceRecordNotFoundWrapper(),
	}
}

// Execute - сворачивает пакет сообщений об активности и обновляет хранилища статистики,
// активности сессий и журнал активности.
func (uc *UserStatistic) Execute(ctx context.Context, messages []dto.UserActivityLogMessage) error {
	if len(messages) == 0 {
		return nil
	}

	slices.SortFunc(messages, func(a, b dto.UserActivityLogMessage) int {
		if cmp := bytes.Compare(a.UserID[:], b.UserID[:]); cmp != 0 {
			return cmp
		}

		return b.VisitedAt.Compare(a.VisitedAt) // сортировка по времени в обратном порядке
	})

	// после сортировки для каждого пользователя первым идёт самое позднее посещение,
	// поэтому берётся первая запись из группы одинаковых UserID
	stat := make([]dto.UserActivityLastVisited, 0, len(messages))

	for i := range messages {
		if len(stat) > 0 && messages[i].UserID == stat[len(stat)-1].UserID {
			continue
		}

		stat = append(
			stat,
			dto.UserActivityLastVisited{
				UserID:        messages[i].UserID,
				LastVisitedAt: messages[i].VisitedAt,
			},
		)
	}

	// целостность данных не критична, поэтому единой транзакции не требуется

	if err := uc.storageSession.UpdateLastActivity(ctx, uc.sessionsLastActivity(messages)); err != nil {
		return uc.errorWrapper.Wrap(err)
	}

	if err := uc.storageActivityStat.UpdateLastVisited(ctx, stat); err != nil {
		return uc.errorWrapper.Wrap(err)
	}

	if err := uc.storageActivityLog.Insert(ctx, messages); err != nil {
		return uc.errorWrapper.Wrap(err)
	}

	return nil
}

// sessionsLastActivity - сворачивает сообщения в последнюю активность по каждой сессии
// (берётся самое позднее посещение); сообщения без сессии (SessionID == 0) пропускаются.
func (uc *UserStatistic) sessionsLastActivity(messages []dto.UserActivityLogMessage) []dto.SessionLastActivity {
	type sessionKey struct {
		userID    uuid.UUID
		sessionID uint32
	}

	latest := make(map[sessionKey]dto.SessionLastActivity, len(messages))

	for _, msg := range messages {
		if msg.SessionID == 0 {
			continue
		}

		realIP, _, err := msg.UserIP.ToUint()
		if err != nil {
			continue // IP не распознан - пропускаем обновление этой записи
		}

		key := sessionKey{
			userID:    msg.UserID,
			sessionID: msg.SessionID,
		}

		if cur, ok := latest[key]; ok && !msg.VisitedAt.After(cur.LastVisitedAt) {
			continue
		}

		latest[key] = dto.SessionLastActivity{
			UserID:        msg.UserID,
			SessionID:     msg.SessionID,
			LastIP:        realIP,
			LastVisitedAt: msg.VisitedAt,
		}
	}

	rows := make([]dto.SessionLastActivity, 0, len(latest))

	for _, row := range latest {
		rows = append(rows, row)
	}

	return rows
}
