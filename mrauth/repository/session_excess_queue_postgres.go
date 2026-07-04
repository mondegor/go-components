package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"

	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// SessionExcessQueuePostgres - очередь пользователей на фоновую чистку лишних сессий в
	// PostgreSQL. Дедуплицируется по (user_id, realm_id): повторная постановка обновляет лимит
	// на значение последнего логина. Решение «какие сессии лишние» принимает обработчик (SessionExcessTrimmer).
	SessionExcessQueuePostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper errors.Wrapper
		tableName    string
	}
)

// NewSessionExcessQueuePostgres - создаёт объект SessionExcessQueuePostgres.
func NewSessionExcessQueuePostgres(
	client mrstorage.DBConnManager,
	tableName string,
) *SessionExcessQueuePostgres {
	return &SessionExcessQueuePostgres{
		client:       client,
		errorWrapper: errors.NewInfraStorageWrapper(),
		tableName:    tableName,
	}
}

// Enqueue - ставит пользователя в очередь на чистку лишних сессий его realm.
// Повтор по той же паре (user_id, realm) обновляет session_max значением последнего логина.
func (re *SessionExcessQueuePostgres) Enqueue(ctx context.Context, userID uuid.UUID, realmID uint16, sessionMax int) error {
	sql := `
		INSERT INTO ` + re.tableName + `
			(user_id, realm_id, session_max)
		VALUES
			($1, $2, $3)
		ON CONFLICT
			(user_id, realm_id) DO UPDATE
		SET
			session_max = EXCLUDED.session_max;`

	_, err := re.client.Conn(ctx).ExecAffected(
		ctx,
		sql,
		userID,
		realmID,
		sessionMax,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// Fetch - выбирает пачку пользователей из очереди (не более limit, старейшие сначала).
// ORDER BY created_at идёт по индексу ix_sessions_excess_queue_created_at, поэтому под большим
// backlog'ом выборка не деградирует в seq-scan по разрастающейся таблице.
//
// Рассчитано на ОДНОГО воркера (single-pod): конкурентной защиты на выборке нет, при нескольких
// воркерах они выбрали бы одну и ту же пачку и продублировали работу (данные не побьются -
// обработка и ack идемпотентны, - но работа удвоится). FOR UPDATE SKIP LOCKED здесь намеренно НЕ
// используется: он потребовал бы держать пачку в одной транзакции до ack, что ломает модель
// consumer-обработчик (Fetch и ack - отдельные шаги). Для мульти-пода нужно добавить в таблицу
// очереди колонку статуса (READY/PROCESSING) и менять её при захвате/завершении пачки, как в mrqueue.
func (re *SessionExcessQueuePostgres) Fetch(ctx context.Context, limit int) ([]entity.SessionExcessItem, error) {
	sql := `
		SELECT
			user_id,
			realm_id,
			session_max
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

	items := make([]entity.SessionExcessItem, 0, limit)

	for cursor.Next() {
		var item entity.SessionExcessItem

		if err = cursor.Scan(&item.UserID, &item.RealmID, &item.SessionMax); err != nil {
			return nil, re.errorWrapper.Wrap(err)
		}

		items = append(items, item)
	}

	if err = cursor.Err(); err != nil {
		return nil, re.errorWrapper.Wrap(err)
	}

	return items, nil
}

// Delete - удаляет указанные пары (user_id, realm) из очереди (ack обработанной пачки). Идемпотентна.
func (re *SessionExcessQueuePostgres) Delete(ctx context.Context, keys []entity.SessionExcessPK) error {
	if len(keys) == 0 {
		return nil
	}

	userIDs := make([]uuid.UUID, 0, len(keys))
	realmIDs := make([]uint16, 0, len(keys))

	for _, key := range keys {
		userIDs = append(userIDs, key.UserID)
		realmIDs = append(realmIDs, key.RealmID)
	}

	sql := `
		DELETE FROM
			` + re.tableName + ` q
		USING
			UNNEST($1::uuid[], $2::int4[]) as t(user_id, realm_id)
		WHERE
			q.user_id = t.user_id AND q.realm_id = t.realm_id;`

	_, err := re.client.Conn(ctx).ExecAffected(
		ctx,
		sql,
		userIDs,
		realmIDs,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}
