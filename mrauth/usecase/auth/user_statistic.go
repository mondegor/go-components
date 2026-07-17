package auth

import (
	"bytes"
	"cmp"
	"context"
	"slices"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrlog"

	"github.com/mondegor/go-components/mrauth/dto"
)

type (
	// UserStatistic - обновляет статистику активности пользователей пакетом сообщений:
	// последнее посещение, активность по сессиям и журнал активности.
	UserStatistic struct {
		storageActivityStat userActivityStatUpdater
		storageActivityLog  userActivityLogStorage
		storageSession      sessionLastActivityUpdater
		logger              mrlog.Logger
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
	logger mrlog.Logger,
) *UserStatistic {
	return &UserStatistic{
		storageActivityStat: storageActivityStat,
		storageActivityLog:  storageActivityLog,
		storageSession:      storageSession,
		logger:              logger,
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
		if c := bytes.Compare(a.UserID[:], b.UserID[:]); c != 0 {
			return c
		}

		if c := cmp.Compare(a.RealmID, b.RealmID); c != 0 {
			return c
		}

		return b.VisitedAt.Compare(a.VisitedAt) // сортировка по времени в обратном порядке
	})

	// после сортировки для каждой пары (пользователь, realm) первым идёт самое позднее
	// посещение, поэтому берётся первая запись из группы одинаковых (UserID, RealmID)
	stat := make([]dto.UserActivityLastVisited, 0, len(messages))

	for i := range messages {
		// RealmID = 0 - realm не определён (см. dto.UserActivityLogMessage): строки статистики
		// для него не существует, обновлять нечего; сессия и журнал обрабатываются как обычно
		if messages[i].RealmID == 0 {
			continue
		}

		if len(stat) > 0 {
			last := stat[len(stat)-1]
			if messages[i].UserID == last.UserID && messages[i].RealmID == last.RealmID {
				continue
			}
		}

		stat = append(
			stat,
			dto.UserActivityLastVisited{
				UserID:        messages[i].UserID,
				RealmID:       messages[i].RealmID,
				LastVisitedAt: messages[i].VisitedAt,
			},
		)
	}

	// целостность данных не критична, поэтому единой транзакции не требуется

	if err := uc.storageSession.UpdateLastActivity(ctx, uc.sessionsLastActivity(messages)); err != nil {
		return uc.errorWrapper.Wrap(err)
	}

	if err := uc.storageActivityStat.UpdateLastVisited(ctx, stat); err != nil {
		if !errors.Is(err, errors.ErrEventStorageRecordsNotAffected) {
			return uc.errorWrapper.Wrap(err)
		}

		// ни одна пара (user, realm) пакета не имеет строки статистики: для realm != 0 строка
		// создаётся при входе в realm (OpenSession, где сбой её записи лишь логируется), поэтому
		// total-miss - признак деградации, а не штатный случай. Сигналим в лог, но пакет
		// не проваливаем: иначе он бесконечно ретраился бы, а keep-alive сессий уже обновлён
		uc.logger.Warn(ctx, "UserStatistic: last-visited batch affected no rows", "pairs", len(stat))
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

		key := sessionKey{
			userID:    msg.UserID,
			sessionID: msg.SessionID,
		}

		if cur, ok := latest[key]; ok && !msg.VisitedAt.After(cur.LastVisitedAt) {
			continue
		}

		latest[key] = dto.SessionLastActivity{
			UserID:    msg.UserID,
			SessionID: msg.SessionID,
			// инвариант: real IP в сообщении всегда задан (источник RemoteAddr,
			// см. produce.UserRequest.Emit), поэтому запись в sessions.last_ip (NOT NULL)
			// безопасна и проверки IsValid не требует
			LastIP:        msg.UserIP.Real,
			LastVisitedAt: msg.VisitedAt,
		}
	}

	rows := make([]dto.SessionLastActivity, 0, len(latest))

	for _, row := range latest {
		rows = append(rows, row)
	}

	return rows
}
