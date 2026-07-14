package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// UserActivityStatPostgres - хранилище статистики активности пользователей в PostgreSQL.
	UserActivityStatPostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper errors.Wrapper
		tableName    string
	}
)

// NewUserActivityStatPostgres - создаёт объект UserActivityStatPostgres.
func NewUserActivityStatPostgres(
	client mrstorage.DBConnManager,
	tableName string,
) *UserActivityStatPostgres {
	return &UserActivityStatPostgres{
		client:       client,
		errorWrapper: errors.NewInfraStorageWrapper(),
		tableName:    tableName,
	}
}

// FetchOne - возвращает запись статистики активности пользователя по его ID.
func (re *UserActivityStatPostgres) FetchOne(ctx context.Context, userID uuid.UUID) (row entity.UserActivityStat, err error) {
	sql := `
		SELECT
			last_login_ip,
			last_logged_at,
			last_visited_at
		FROM
			` + re.tableName + `
		WHERE
			user_id = $1
		LIMIT 1;`

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		userID,
	).Scan(
		&row.LastLoginIP,
		&row.LastLoggedAt,
		&row.LastVisitedAt,
	)
	if err != nil {
		return entity.UserActivityStat{}, re.errorWrapper.Wrap(err)
	}

	return row, nil
}

// InsertOrUpdate - создаёт или обновляет запись статистики активности пользователя.
func (re *UserActivityStatPostgres) InsertOrUpdate(ctx context.Context, row entity.UserActivityStat) error {
	sql := `
		INSERT INTO ` + re.tableName + `
			(
				user_id,
				last_login_ip,
				last_logged_at,
				last_visited_at
			)
		VALUES
			($1, $2, $3, $4)
		ON CONFLICT
			(user_id) DO UPDATE
		SET
			last_login_ip = EXCLUDED.last_login_ip,
			last_logged_at = EXCLUDED.last_logged_at,
			last_visited_at = EXCLUDED.last_visited_at;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		row.UserID,
		row.LastLoginIP,
		row.LastLoggedAt,
		row.LastVisitedAt,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}

// UpdateLastVisited - батчем обновляет время последнего визита (last_visited_at).
// Поле last_visited_at не будет обновлено в меньшую сторону.
func (re *UserActivityStatPostgres) UpdateLastVisited(ctx context.Context, rows []dto.UserActivityLastVisited) error {
	if len(rows) == 0 {
		return nil
	}

	userIDs := make([]uuid.UUID, 0, len(rows))
	visitedAts := make([]time.Time, 0, len(rows))

	for _, row := range rows {
		userIDs = append(userIDs, row.UserID)
		visitedAts = append(visitedAts, row.LastVisitedAt)
	}

	sql := `
		UPDATE
			` + re.tableName + ` t1
		SET
			last_visited_at = GREATEST(t1.last_visited_at, t2.last_visited_at)
		FROM
		  	(
				SELECT *
				FROM
					UNNEST($1::uuid[], $2::timestamptz[])
					as t(user_id, last_visited_at)
			) t2
		WHERE
			t1.user_id = t2.user_id;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		userIDs,
		visitedAts,
	)
	if err != nil {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}
