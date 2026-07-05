package clean

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/util/slices/ordered"

	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// SessionExcessTrimmer - воркер фоновой чистки лишних сессий. По каждой записи очереди
	// (пользователь+realm) пересчитывает живые сессии этого realm и, ТОЛЬКО при превышении лимита,
	// ревокает дубли одного устройства и наименее активные сверх лимита, затем сам удаляет
	// осиротевшие строки.
	SessionExcessTrimmer struct {
		txManager    mrstorage.DBTxManager
		consumer     SessionExcessQueueConsumer
		openFetcher  OpenSessionFetcher
		lister       SessionLister
		closer       SessionCloser
		deleter      OrphanSessionDeleter
		errorWrapper errors.Wrapper
	}

	// SessionExcessQueueConsumer - очередь чистки лишних сессий: выборка и подтверждение пачки
	// записей (пар пользователь+realm).
	SessionExcessQueueConsumer interface {
		Fetch(ctx context.Context, limit int) ([]entity.SessionExcessItem, error)
		Delete(ctx context.Context, keys []entity.SessionExcessPK) error
	}

	// OpenSessionFetcher - выборка идентификаторов открытых сессий пользователя в realm.
	OpenSessionFetcher interface {
		FetchOpenSessionIDs(ctx context.Context, userID uuid.UUID, realmID uint16) (sessionIDs []uint32, err error)
	}

	// SessionLister - выборка упорядоченного списка сессий пользователя.
	SessionLister interface {
		FetchOrderedListByUserIDAndSessionIDs(ctx context.Context, userID uuid.UUID, sessionIDs []uint32, limit int) ([]entity.Session, error)
	}

	// SessionCloser - ревок токенов указанных сессий пользователя.
	SessionCloser interface {
		RevokeTokensBySessionIDs(ctx context.Context, userID uuid.UUID, sessionIDs []uint32) error
	}
)

// NewSessionExcessTrimmer - создаёт объект SessionExcessTrimmer.
func NewSessionExcessTrimmer(
	txManager mrstorage.DBTxManager,
	consumer SessionExcessQueueConsumer,
	openFetcher OpenSessionFetcher,
	lister SessionLister,
	closer SessionCloser,
	deleter OrphanSessionDeleter,
) *SessionExcessTrimmer {
	return &SessionExcessTrimmer{
		txManager:    txManager,
		consumer:     consumer,
		openFetcher:  openFetcher,
		lister:       lister,
		closer:       closer,
		deleter:      deleter,
		errorWrapper: errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - обрабатывает одну пачку очереди (до limit пользователей) и возвращает её размер
// (для ItemBatchPlayer: count < limit = очередь иссякла). Возвращается размер пачки, а не число
// ревокнутых сессий: пользователи без лишних сессий не должны обрывать цикл раньше времени.
// Надёжность (at-least-once): ack (consumer.Delete) делается ПОСЛЕ обработки всей пачки, поэтому
// краш между обработкой и ack приводит лишь к идемпотентной переобработке, без потерь.
//
// Пользователи пачки обрабатываются ПОСЛЕДОВАТЕЛЬНО (без внутрибатчевого параллелизма): на каждого
// приходится ~3-4 round-trip'а к БД (FetchOpenSessionIDs + FetchOrderedList + транзакция ревока),
// то есть до ~limit*4 запросов на пачку. Это осознанный trade-off ради простоты single-pod-воркера:
// длительность ограничена durationLimit у ItemBatchPlayer, а крупный backlog разгребается за несколько
// циклов. Если понадобится ускорить - батчить выборки по группе user_id (user_id = ANY(...)),
// оставив транзакцию ревока пер-юзерной.
// TODO: при росте backlog'а батчить read-сторону (FetchOpenSessionIDs/FetchOrderedList) по группе
// user_id = ANY(...), сократив ~limit*4 round-trip'ов, транзакцию ревока оставить пер-юзерной.
func (co *SessionExcessTrimmer) Execute(ctx context.Context, limit int) (count int, err error) {
	if limit < 1 {
		return 0, errors.ErrInternalIncorrectInputData.WithDetails("limit is zero or negative")
	}

	items, err := co.consumer.Fetch(ctx, limit)
	if err != nil {
		return 0, co.errorWrapper.Wrap(err)
	}

	if len(items) == 0 {
		return 0, nil
	}

	keys := make([]entity.SessionExcessPK, 0, len(items))

	for _, item := range items {
		if err = co.trimUser(ctx, item); err != nil {
			return 0, co.errorWrapper.Wrap(err)
		}

		keys = append(keys, entity.SessionExcessPK{UserID: item.UserID, RealmID: item.RealmID})
	}

	if err = co.consumer.Delete(ctx, keys); err != nil {
		return 0, co.errorWrapper.Wrap(err)
	}

	return len(items), nil
}

// trimUser - пересчитывает живые сессии пользователя в realm записи, выбирает лишние (дубли
// устройства + сверх лимита) и в одной транзакции ревокает их токены и удаляет осиротевшие строки.
// session_id уникален в пределах пользователя и принадлежит сессиям только этого realm, поэтому
// ревок/удаление по session_id не задевают другие realm. DeleteOrphaned удалит лишь строки без
// живого refresh-токена - защита от гонки с переоткрытием сессии.
func (co *SessionExcessTrimmer) trimUser(ctx context.Context, item entity.SessionExcessItem) error {
	openSessionIDs, err := co.openFetcher.FetchOpenSessionIDs(ctx, item.UserID, item.RealmID)
	if err != nil {
		return err
	}

	if len(openSessionIDs) == 0 {
		return nil
	}

	// limit=0: триммеру нужны ВСЕ открытые сессии (дедуп по устройству и поиск лишних), без обрезки.
	sessions, err := co.lister.FetchOrderedListByUserIDAndSessionIDs(ctx, item.UserID, openSessionIDs, 0)
	if err != nil {
		return err
	}

	toRevoke := selectExcessSessions(sessions, item.SessionMax)
	if len(toRevoke) == 0 {
		return nil
	}

	return co.txManager.Do(ctx, func(ctx context.Context) error {
		if err := co.closer.RevokeTokensBySessionIDs(ctx, item.UserID, toRevoke); err != nil {
			return err
		}

		pks := make([]entity.SessionPK, 0, len(toRevoke))
		for _, sessionID := range toRevoke {
			pks = append(
				pks,
				entity.SessionPK{
					UserID:    item.UserID,
					SessionID: sessionID,
				},
			)
		}

		return co.deleter.DeleteOrphaned(ctx, pks)
	})
}

// selectExcessSessions - выбирает идентификаторы сессий одного пользователя на ревок, оставляя
// ровно limit «лучших» и ревокая ровно excess = len(sessions) - limit лишних, не больше.
// Дедуп устройств и обрезка выполняются ТОЛЬКО при превышении лимита: пока общее число сессий
// не больше лимита, ничего не ревокается, даже если есть дубли одного устройства. Важно и обратное:
// если убрать дубли достаточно, чтобы вернуться к лимиту, оставшиеся дубли НЕ трогаются.
//
// Один проход active-first (порядок гарантирует FetchOrderedListByUserIDAndSessionIDs): дубль
// устройства (UA уже встречался) либо сессия после исчерпания лимита (kept >= limit) попадают в
// candidates с сохранением порядка. В конце среди кандидатов оставляются limit-kept самых активных
// (добивают лимит, если уникальных устройств меньше limit), хвост уходит на ревок: candidates[limit-kept:].
// Пока kept < limit, ВСЕ кандидаты - дубли (сверхлимитные уники появляются только при kept == limit),
// поэтому отбрасывается ровно нужное число наименее активных дублей.
// Пустой user_agent считается отдельным устройством (не дедуплицируется).
func selectExcessSessions(sessions []entity.Session, limit int) []uint32 {
	if limit < 1 {
		limit = 1
	}

	// в пределах лимита сессии не трогаем (в т.ч. дубли устройств): дедуп - механизм обрезки
	// лишнего, а не принудительного «одна сессия на устройство»
	if len(sessions) <= limit {
		return nil
	}

	kept := 0
	seen := make([]string, 0, len(sessions))
	candidates := make([]uint32, 0, len(sessions)-limit)

	for _, session := range sessions {
		// дубль устройства либо сессия сверх лимита - кандидат на ревок (порядок active-first сохраняется)
		if session.UserAgent != "" && ordered.BinaryContains(seen, session.UserAgent) {
			candidates = append(candidates, session.SessionID)

			continue
		}

		if kept >= limit {
			candidates = append(candidates, session.SessionID)

			continue
		}

		if session.UserAgent != "" {
			seen = ordered.UniqueBinaryAppend(seen, session.UserAgent)
		}

		kept++
	}

	// оставляем limit-kept самых активных кандидатов (дубли, добивающие лимит), хвост - на ревок.
	return candidates[limit-kept:]
}
