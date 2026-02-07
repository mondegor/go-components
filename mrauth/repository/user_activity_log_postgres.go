package repository

import (
	"context"
	"strings"
	"time"

	"github.com/mondegor/go-storage/mrpostgres/stream/placeholdedvalues"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth/dto"
)

type (
	// UserActivityLogPostgres - репозиторий для хранения элементов настроек.
	UserActivityLogPostgres struct {
		client           mrstorage.DBConnManager
		errorWrapper     errors.Wrapper
		tableName        string
		insertArgsHelper placeholdedvalues.SQL
	}
)

// NewUserActivityLogPostgres - создаёт объект UserActivityLogPostgres.
func NewUserActivityLogPostgres(
	client mrstorage.DBConnManager,
	tableName string,
) *UserActivityLogPostgres {
	const countLineArgs = 7

	return &UserActivityLogPostgres{
		client:       client,
		errorWrapper: errors.NewInfraStorageWrapper(),
		tableName:    tableName,

		insertArgsHelper: placeholdedvalues.New(
			placeholdedvalues.WithCountLineArgs(countLineArgs),
		),
	}
}

// Insert - фиксирует изменение настройки.
func (re *UserActivityLogPostgres) Insert(ctx context.Context, rows []dto.UserActivityLogMessage) error {
	if len(rows) == 0 {
		return nil
	}

	var sql strings.Builder

	sql.WriteString(`
		INSERT INTO ` + re.tableName + `
			(
				user_id,
				user_ip,
				user_ip_string,
				user_agent,
				request_path,
				request_status,
				visited_at
			)
		VALUES `)

	// generate: ($1, $2, $3, $4, $5, $6, $7), ...
	values := make([]any, 0, len(rows)*re.insertArgsHelper.CountLineArgs())
	argumentNumber := re.insertArgsHelper.WriteFirstLine(&sql)

	for i, row := range rows {
		if i > 0 {
			argumentNumber = re.insertArgsHelper.WriteNextLine(&sql, argumentNumber)
		}

		realIP, _, err := row.UserIP.ToUint()
		if err != nil {
			return err // TODO: можно логировать ошибку
		}

		values = append(
			values,
			row.UserID, realIP, row.UserIP.String(), row.UserAgent, row.RequestPath, row.RequestStatus, row.VisitedAt,
		)
	}

	sql.WriteByte(';')

	return re.client.Conn(ctx).Exec(
		ctx,
		sql.String(),
		values...,
	)
}

// DeleteBeforeDate - comments method.
func (re *UserActivityLogPostgres) DeleteBeforeDate(ctx context.Context, datetime time.Time, limit int) error {
	sql := `
		WITH old_items as (
			SELECT
			  	record_id as item_id
			FROM
			  	` + re.tableName + `
			WHERE
				visited_at < $1
			ORDER BY
				visited_at ASC
		    LIMIT $2
		)
		DELETE FROM
			` + re.tableName + ` t1
		USING
			old_items ei
		WHERE
			t1.record_id = ei.item_id;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		datetime,
		limit,
	)
	// если это внутренняя ошибка
	if err != nil && !errors.Is(err, errors.ErrEventStorageRowsNotAffected) {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}
