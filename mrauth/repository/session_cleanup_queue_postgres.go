package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"

	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// SessionCleanupQueuePostgres - очередь (пары user_id, session_id) на очистку сессий
	// в PostgreSQL. Generic-консумер: умеет только ставить пары в очередь, выбирать пачку
	// и удалять обработанную пачку. Решение «какая сессия осиротела» лежит на обработчике.
	SessionCleanupQueuePostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper errors.Wrapper
		tableName    string
	}
)

// NewSessionCleanupQueuePostgres - создаёт объект SessionCleanupQueuePostgres.
func NewSessionCleanupQueuePostgres(
	client mrstorage.DBConnManager,
	tableName string,
) *SessionCleanupQueuePostgres {
	return &SessionCleanupQueuePostgres{
		client:       client,
		errorWrapper: errors.NewInfraStorageWrapper(),
		tableName:    tableName,
	}
}

// Enqueue - ставит пары (user_id, session_id) в очередь на удаление.
// Повторы игнорируются - очередь дедуплицируется по PK.
func (re *SessionCleanupQueuePostgres) Enqueue(ctx context.Context, pks []entity.SessionPK) error {
	if len(pks) == 0 {
		return nil
	}

	userIDs := make([]uuid.UUID, 0, len(pks))
	sessionIDs := make([]uint32, 0, len(pks))

	for _, pk := range pks {
		userIDs = append(userIDs, pk.UserID)
		sessionIDs = append(sessionIDs, pk.SessionID)
	}

	sql := `
		INSERT INTO ` + re.tableName + `
			(user_id, session_id)
		SELECT
			user_id, session_id
		FROM
			UNNEST($1::uuid[], $2::int8[]) as t(user_id, session_id)
		ON CONFLICT
			(user_id, session_id) DO NOTHING;`

	_, err := re.client.Conn(ctx).ExecAffected(
		ctx,
		sql,
		userIDs,
		sessionIDs,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// Fetch - выбирает пачку кандидатов сессий на удаление из очереди (не более limit, старейшие сначала).
// ORDER BY created_at идёт по индексу ix_sessions_cleanup_queue_created_at, поэтому под большим
// backlog'ом выборка не деградирует в seq-scan по разрастающейся таблице.
//
// Рассчитано на ОДНОГО воркера (single-pod): конкурентной защиты на выборке нет, при нескольких
// воркерах они выбрали бы одну и ту же пачку и продублировали работу (данные не побьются -
// обработка и ack идемпотентны, - но работа удвоится). FOR UPDATE SKIP LOCKED здесь намеренно НЕ
// используется: он потребовал бы держать пачку в одной транзакции до ack, что ломает модель
// consumer-обработчик (Fetch и ack - отдельные шаги). Для мульти-пода нужно добавить в таблицу
// очереди колонку статуса (READY/PROCESSING) и менять её при захвате/завершении пачки, как в mrqueue.
func (re *SessionCleanupQueuePostgres) Fetch(ctx context.Context, limit int) ([]entity.SessionPK, error) {
	sql := `
		SELECT
			user_id,
			session_id
		FROM
			` + re.tableName + `
		ORDER BY
			created_at
		` + mrstorage.NonZeroLimit(limit) + `;`

	cursor, err := re.client.Conn(ctx).Query(ctx, sql)
	if err != nil {
		return nil, re.errorWrapper.Wrap(err)
	}

	defer cursor.Close()

	pks := make([]entity.SessionPK, 0, limit)

	for cursor.Next() {
		var pk entity.SessionPK

		if err = cursor.Scan(&pk.UserID, &pk.SessionID); err != nil {
			return nil, re.errorWrapper.Wrap(err)
		}

		pks = append(pks, pk)
	}

	if err = cursor.Err(); err != nil {
		return nil, re.errorWrapper.Wrap(err)
	}

	return pks, nil
}

// Delete - удаляет указанные пары из очереди (ack обработанной пачки). Идемпотентна.
func (re *SessionCleanupQueuePostgres) Delete(ctx context.Context, pks []entity.SessionPK) error {
	if len(pks) == 0 {
		return nil
	}

	userIDs := make([]uuid.UUID, 0, len(pks))
	sessionIDs := make([]uint32, 0, len(pks))

	for _, pk := range pks {
		userIDs = append(userIDs, pk.UserID)
		sessionIDs = append(sessionIDs, pk.SessionID)
	}

	sql := `
		DELETE FROM
			` + re.tableName + ` q
		USING
			UNNEST($1::uuid[], $2::int8[]) as t(user_id, session_id)
		WHERE
			q.user_id = t.user_id AND q.session_id = t.session_id;`

	_, err := re.client.Conn(ctx).ExecAffected(
		ctx,
		sql,
		userIDs,
		sessionIDs,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}
