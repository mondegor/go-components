package repository

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrpostgres/stream/placeholdedvalues"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrtype"

	core "github.com/mondegor/go-components/internal"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// UserActivityStatPostgres - репозиторий для хранения сообщений подготовленных для отправки различным получателям.
	UserActivityStatPostgres struct {
		client       mrstorage.DBConnManager
		table        mrsql.DBTableInfo
		errorWrapper core.ErrorWrapper
	}
)

// NewUserActivityStatPostgres - создаёт объект UserActivityStatPostgres.
func NewUserActivityStatPostgres(client mrstorage.DBConnManager, table mrsql.DBTableInfo) *UserActivityStatPostgres {
	return &UserActivityStatPostgres{
		client:       client,
		table:        table,
		errorWrapper: core.NewStorageErrorWrapper(table.Name),
	}
}

// FetchOne - возвращает список сообщений по их указанным SettingID.
func (re *UserActivityStatPostgres) FetchOne(ctx context.Context, userID uuid.UUID) (row entity.UserActivityStat, err error) {
	sql := `
		SELECT
			last_login_ip,
			last_logged_at,
			last_visited_at
		FROM
			` + re.table.Name + `
		WHERE
			` + re.table.PrimaryKey + ` = $1
		LIMIT 1;`

	var lastLoginIP uint32

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		userID,
	).Scan(
		&lastLoginIP,
		&row.LastLoggedAt,
		&row.LastVisitedAt,
	)
	if err != nil {
		return entity.UserActivityStat{}, re.errorWrapper.WrapError(err)
	}

	row.LastLoginIP = mrtype.NewDetailedIP(lastLoginIP, 0)

	return row, nil
}

// InsertOrUpdate - возвращает список сообщений по их указанным SettingID.
func (re *UserActivityStatPostgres) InsertOrUpdate(ctx context.Context, row entity.UserActivityStat) error {
	sql := `
		INSERT INTO ` + re.table.Name + `
			(
				` + re.table.PrimaryKey + `,
				last_login_ip,
				last_login_ip_string,
				last_logged_at,
				last_visited_at
			)
		VALUES
			($1, $2, $3, $4, $5)
		ON CONFLICT (` + re.table.PrimaryKey + `) DO UPDATE
		SET
			last_login_ip = EXCLUDED.last_login_ip,
			last_login_ip_string = EXCLUDED.last_login_ip_string,
			last_logged_at = EXCLUDED.last_logged_at,
			last_visited_at = EXCLUDED.last_visited_at;`

	realIP, _, err := row.LastLoginIP.ToUint()
	if err != nil {
		return err // TODO: можно логировать ошибку
	}

	err = re.client.Conn(ctx).Exec(
		ctx,
		sql,
		row.UserID,
		realIP,
		row.LastLoginIP.String(),
		row.LastLoggedAt,
		row.LastVisitedAt,
	)
	if err != nil {
		return re.errorWrapper.WrapError(err)
	}

	return nil
}

// UpdateLastVisited - фиксирует изменение настройки.
// Поле last_visited_at не будет обновлено в меньшую сторону.
func (re *UserActivityStatPostgres) UpdateLastVisited(ctx context.Context, rows []entity.UserActivityLastVisited) error {
	if len(rows) == 0 {
		return nil
	}

	var sql strings.Builder

	sql.WriteString(`
		UPDATE
			` + re.table.Name + ` t1
		SET
			last_visited_at = GREATEST(t1.last_visited_at, t2.last_visited_at)
		FROM (
			VALUES
		`)

	const countLineArgs = 2

	// generate: ($1::uuid, $2::timestamptz), ...
	sqlValues := placeholdedvalues.New(
		&sql,
		placeholdedvalues.WithLineMiddle(map[int]string{1: "::uuid, "}),
		placeholdedvalues.WithLinePostfix("::timestamptz"),
		placeholdedvalues.WithCountArgs(countLineArgs),
	)

	values := make([]any, 0, len(rows)*countLineArgs)
	argumentNumber := sqlValues.WriteFirstLine()

	for i, row := range rows {
		if i > 0 {
			argumentNumber = sqlValues.WriteNextLine(argumentNumber)
		}

		values = append(
			values,
			row.UserID, row.LastVisitedAt,
		)
	}

	sql.WriteString(`
			) as t2 (user_id, last_visited_at)
		WHERE
			t1.` + re.table.PrimaryKey + ` = t2.user_id;`)

	return re.client.Conn(ctx).Exec(
		ctx,
		sql.String(),
		values...,
	)
}
