package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// UserSecurityLogPostgres - репозиторий журнала безопасности пользователей.
	UserSecurityLogPostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper errors.Wrapper
		tableName    string
	}
)

// NewUserSecurityLogPostgres - создаёт объект UserSecurityLogPostgres.
func NewUserSecurityLogPostgres(
	client mrstorage.DBConnManager,
	tableName string,
) *UserSecurityLogPostgres {
	return &UserSecurityLogPostgres{
		client:       client,
		errorWrapper: errors.NewInfraStorageWrapper(),
		tableName:    tableName,
	}
}

// Insert - фиксирует запись журнала безопасности (в транзакции вызывающего, если она открыта).
func (re *UserSecurityLogPostgres) Insert(ctx context.Context, row entity.SecurityLogEvent) error {
	sql := `
		INSERT INTO ` + re.tableName + `
			(
				user_id,
				event_type,
				client_ip,
				client_proxy_ip,
				user_agent,
				event_extra
			)
		VALUES
			($1, $2, $3, $4, $5, $6);`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		row.UserID,
		row.EventType,
		row.ClientIP.Real,
		row.ClientIP.Proxy,
		row.UserAgent,
		row.Extra,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// FetchByUserID - возвращает записи журнала безопасности пользователя от самой свежей к старым,
// начиная с позиции cursor (не более cursor.Limit), и признак наличия записей за ней.
func (re *UserSecurityLogPostgres) FetchByUserID(
	ctx context.Context,
	userID uuid.UUID,
	cursor mrstorage.IDCursor,
) (rows []entity.SecurityLogEvent, hasNext bool, err error) {
	sql := `
		SELECT
			record_id,
			event_type,
			client_ip,
			client_proxy_ip,
			user_agent,
			event_extra,
			created_at
		FROM
			` + re.tableName + `
		WHERE
			user_id = $1 AND record_id < $2
		ORDER BY
			record_id DESC
		` + mrstorage.NonZeroLimit(cursor.Limit+1) + `;`

	dbCursor, err := re.client.Conn(ctx).Query(
		ctx,
		sql,
		userID,
		cursor.BeforeID(),
	)
	if err != nil {
		return nil, false, re.errorWrapper.Wrap(err)
	}

	defer dbCursor.Close()

	rows = make([]entity.SecurityLogEvent, 0, cursor.Limit)

	for dbCursor.Next() {
		// лишняя строка сверх limit лишь сообщает, что за текущей страницей есть записи
		if len(rows) == cursor.Limit {
			hasNext = true

			break
		}

		row := entity.SecurityLogEvent{
			UserID: userID,
		}

		if err = dbCursor.Scan(
			&row.RecordID,
			&row.EventType,
			&row.ClientIP.Real,
			&row.ClientIP.Proxy,
			&row.UserAgent,
			&row.Extra,
			&row.CreatedAt,
		); err != nil {
			return nil, false, re.errorWrapper.Wrap(err)
		}

		// системное время: домен всегда оперирует UTC независимо от зоны сессии БД
		row.CreatedAt = row.CreatedAt.UTC()

		rows = append(rows, row)
	}

	if err = dbCursor.Err(); err != nil {
		return nil, false, re.errorWrapper.Wrap(err)
	}

	return rows, hasNext, nil
}

// DeleteBeforeDate - удаляет пачку записей журнала старше datetime (не более limit)
// и возвращает число фактически удалённых строк (сигнал "пачка была полной, есть ещё").
// Рассчитано на single-pod-планировщик (см. wire/mrauth/scheduler.NewService): конкурентной защиты на выборке нет.
func (re *UserSecurityLogPostgres) DeleteBeforeDate(ctx context.Context, datetime time.Time, limit int) (count int, err error) {
	sql := `
		DELETE FROM
			` + re.tableName + ` t1
		USING (
			SELECT
				record_id
			FROM
				` + re.tableName + `
			WHERE
				created_at < $1
			ORDER BY
				created_at ASC
			` + mrstorage.NonZeroLimit(limit) + `
		) ei
		WHERE
			t1.record_id = ei.record_id;`

	count, err = re.client.Conn(ctx).ExecAffected(
		ctx,
		sql,
		datetime,
	)
	if err != nil {
		return 0, re.errorWrapper.Wrap(err)
	}

	return count, nil
}
