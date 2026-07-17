package repository

import (
	"context"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// SessionPostgres - хранилище пользовательских сессий в PostgreSQL.
	SessionPostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper errors.Wrapper
		tableName    string
	}
)

// NewSessionPostgres - создаёт объект SessionPostgres.
func NewSessionPostgres(
	client mrstorage.DBConnManager,
	tableName string,
) *SessionPostgres {
	return &SessionPostgres{
		client:       client,
		errorWrapper: errors.NewInfraStorageWrapper(),
		tableName:    tableName,
	}
}

// FetchOrderedListByUserIDAndSessionIDs - возвращает строки метаданных сессий пользователя по
// заданным session_id, упорядоченные по последней активности (updated_at DESC, при равенстве -
// больший session_id вперёд для детерминированности). Набор ограничивается переданными
// идентификаторами, что согласует выборку с FetchOpenSessions и убирает неограниченное чтение.
func (re *SessionPostgres) FetchOrderedListByUserIDAndSessionIDs(
	ctx context.Context,
	userID uuid.UUID,
	sessionIDs []uint32,
	limit int,
) ([]entity.Session, error) {
	if len(sessionIDs) == 0 {
		return nil, nil
	}

	if limit < 1 {
		limit = len(sessionIDs)
	}

	sql := `
		SELECT
			session_id,
			COALESCE(user_agent, ''),
			last_ip,
			created_at,
			updated_at
		FROM
			` + re.tableName + `
		WHERE
			user_id = $1 AND session_id = ANY($2::int8[])
		ORDER BY
			updated_at DESC, session_id DESC
		` + mrstorage.NonZeroLimit(limit) + `;`

	cursor, err := re.client.Conn(ctx).Query(
		ctx,
		sql,
		userID,
		sessionIDs,
	)
	if err != nil {
		return nil, re.errorWrapper.Wrap(err)
	}

	defer cursor.Close()

	rows := make([]entity.Session, 0)

	for cursor.Next() {
		row := entity.Session{
			UserID: userID,
		}

		if err = cursor.Scan(
			&row.SessionID,
			&row.UserAgent,
			&row.LastIP,
			&row.CreatedAt,
			&row.UpdatedAt,
		); err != nil {
			return nil, re.errorWrapper.Wrap(err)
		}

		rows = append(rows, row)
	}

	if err = cursor.Err(); err != nil {
		return nil, re.errorWrapper.Wrap(err)
	}

	return rows, nil
}

// Insert - сохраняет строку сессии при её открытии с заранее заданным row.SessionID.
// Возможные ошибки:
//   - ErrEventRecordAlreadyExists - такой session_id уже занят (конфликт первичного ключа).
func (re *SessionPostgres) Insert(ctx context.Context, row entity.Session) error {
	sql := `
		INSERT INTO ` + re.tableName + `
			(
				user_id,
				session_id,
				user_agent,
				last_ip,
				updated_at
			)
		VALUES
			($1, $2, $3, $4, NOW())
		ON CONFLICT
			(user_id, session_id) DO NOTHING;`

	// при ON CONFLICT DO NOTHING конфликт не вставляет строку: Exec возвращает
	// ErrEventStorageRecordsNotAffected, что трактуется как коллизия session_id.
	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		row.UserID,
		row.SessionID,
		row.UserAgent,
		row.LastIP,
	)
	if err != nil {
		if errors.Is(err, errors.ErrEventStorageRecordsNotAffected) {
			return errors.ErrEventRecordAlreadyExists
		}

		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// UpdateLastActivity - батчем обновляет последнюю активность сессий (IP и время).
// Целостность не критична: записи без совпадающей сессии просто игнорируются.
func (re *SessionPostgres) UpdateLastActivity(ctx context.Context, rows []dto.SessionLastActivity) error {
	if len(rows) == 0 {
		return nil
	}

	userIDs := make([]uuid.UUID, 0, len(rows))
	sessionIDs := make([]uint32, 0, len(rows))
	lastIPs := make([]netip.Addr, 0, len(rows))
	visitedAts := make([]time.Time, 0, len(rows))

	for _, row := range rows {
		userIDs = append(userIDs, row.UserID)
		sessionIDs = append(sessionIDs, row.SessionID)
		lastIPs = append(lastIPs, row.LastIP)
		visitedAts = append(visitedAts, row.LastVisitedAt)
	}

	sql := `
		UPDATE
			` + re.tableName + ` t1
		SET
			last_ip = CASE WHEN t2.updated_at >= t1.updated_at THEN t2.last_ip ELSE t1.last_ip END,
			updated_at = GREATEST(t1.updated_at, t2.updated_at)
		FROM
			(
				SELECT *
				FROM
					UNNEST($1::uuid[], $2::int8[], $3::inet[], $4::timestamptz[])
					as t(user_id, session_id, last_ip, updated_at)
			) t2
		WHERE
			t1.user_id = t2.user_id AND t1.session_id = t2.session_id;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		userIDs,
		sessionIDs,
		lastIPs,
		visitedAts,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}
