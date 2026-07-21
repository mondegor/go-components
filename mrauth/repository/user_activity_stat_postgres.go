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

// Fetch - возвращает записи статистики активности пользователя по всем его realm'ам.
func (re *UserActivityStatPostgres) Fetch(ctx context.Context, userID uuid.UUID) ([]entity.UserActivityStat, error) {
	sql := `
		SELECT
			realm_id,
			last_login_ip,
			last_logged_at,
			last_visited_at
		FROM
			` + re.tableName + `
		WHERE
			user_id = $1
		ORDER BY
			realm_id ASC;`

	cursor, err := re.client.Conn(ctx).Query(
		ctx,
		sql,
		userID,
	)
	if err != nil {
		return nil, re.errorWrapper.Wrap(err)
	}

	defer cursor.Close()

	rows := make([]entity.UserActivityStat, 0)

	for cursor.Next() {
		row := entity.UserActivityStat{
			UserID: userID,
		}

		err = cursor.Scan(
			&row.RealmID,
			&row.LastLoginIP,
			&row.LastLoggedAt,
			&row.LastVisitedAt,
		)
		if err != nil {
			return nil, re.errorWrapper.Wrap(err)
		}

		// системное время: домен всегда оперирует UTC независимо от зоны сессии БД
		row.LastLoggedAt = row.LastLoggedAt.UTC()
		row.LastVisitedAt = row.LastVisitedAt.UTC()

		rows = append(rows, row)
	}

	if err = cursor.Err(); err != nil {
		return nil, re.errorWrapper.Wrap(err)
	}

	return rows, nil
}

// InsertOrUpdate - создаёт или обновляет запись статистики активности пользователя.
func (re *UserActivityStatPostgres) InsertOrUpdate(ctx context.Context, row entity.UserActivityStat) error {
	sql := `
		INSERT INTO ` + re.tableName + `
			(
				user_id,
				realm_id,
				last_login_ip,
				last_logged_at,
				last_visited_at
			)
		VALUES
			($1, $2, $3, $4, $5)
		ON CONFLICT
			(user_id, realm_id) DO UPDATE
		SET
			last_login_ip = EXCLUDED.last_login_ip,
			last_logged_at = EXCLUDED.last_logged_at,
			last_visited_at = EXCLUDED.last_visited_at;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		row.UserID,
		row.RealmID,
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
// Если ни одна пара (user, realm) пакета не имеет строки статистики, возвращает
// errors.ErrEventStorageRecordsNotAffected: строка создаётся при входе в realm,
// поэтому total-miss - признак деградации, и решение о нём принимает вызывающий
// (см. auth.UserStatistic.Execute).
func (re *UserActivityStatPostgres) UpdateLastVisited(ctx context.Context, rows []dto.UserActivityLastVisited) error {
	if len(rows) == 0 {
		return nil
	}

	userIDs := make([]uuid.UUID, 0, len(rows))
	realmIDs := make([]uint16, 0, len(rows))
	visitedAts := make([]time.Time, 0, len(rows))

	for _, row := range rows {
		userIDs = append(userIDs, row.UserID)
		realmIDs = append(realmIDs, row.RealmID)
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
					UNNEST($1::uuid[], $2::int4[], $3::timestamptz[])
					as t(user_id, realm_id, last_visited_at)
			) t2
		WHERE
			t1.user_id = t2.user_id AND t1.realm_id = t2.realm_id;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		userIDs,
		realmIDs,
		visitedAts,
	)
	if err != nil {
		// сентинел "ни одна строка не затронута" пробрасывается без оборачивания,
		// чтобы вызывающий мог отличить total-miss от настоящей ошибки запроса
		if errors.Is(err, errors.ErrEventStorageRecordsNotAffected) {
			return err
		}

		return re.errorWrapper.Wrap(err)
	}

	return nil
}
