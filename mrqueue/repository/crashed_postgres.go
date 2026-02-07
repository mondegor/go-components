package repository

import (
	"context"
	"strings"
	"time"

	"github.com/mondegor/go-storage/mrpostgres/stream/placeholdedvalues"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"

	"github.com/mondegor/go-components/mrqueue/entity"
)

type (
	// CrashedPostgres - репозиторий для хранения ошибок записей, которые случились при их обработке.
	CrashedPostgres struct {
		client           mrstorage.DBConnManager
		table            mrsql.DBTableInfo
		insertArgsHelper placeholdedvalues.SQL
	}
)

// NewCrashedPostgres - создаёт объект CrashedPostgres.
func NewCrashedPostgres(client mrstorage.DBConnManager, table mrsql.DBTableInfo) *CrashedPostgres {
	const countLineArgs = 2

	return &CrashedPostgres{
		client: client,
		table:  table,

		insertArgsHelper: placeholdedvalues.New(
			placeholdedvalues.WithCountLineArgs(countLineArgs),
		),
	}
}

// Insert - добавляет указанный список записей в журнал ошибок.
func (re *CrashedPostgres) Insert(ctx context.Context, rows []entity.CrashedItem) error {
	if len(rows) == 0 {
		return nil
	}

	var sql strings.Builder

	sql.WriteString(`
		INSERT INTO ` + re.table.Name + `
			(
				` + re.table.PrimaryKey + `,
				error_message
			)
		VALUES `)

	// generate: ($1, $2), ...
	values := make([]any, 0, len(rows)*re.insertArgsHelper.CountLineArgs())
	argumentNumber := re.insertArgsHelper.WriteFirstLine(&sql)

	for i, row := range rows {
		if i > 0 {
			argumentNumber = re.insertArgsHelper.WriteNextLine(&sql, argumentNumber)
		}

		values = append(values, row.ID, row.Cause)
	}

	sql.WriteByte(';')

	return re.client.Conn(ctx).Exec(
		ctx,
		sql.String(),
		values...,
	)
}

// InsertOne - добавляет указанную запись в журнал ошибок.
func (re *CrashedPostgres) InsertOne(ctx context.Context, row entity.CrashedItem) error {
	return re.Insert(ctx, []entity.CrashedItem{row})
}

// Delete - удаляет ограниченный список записей из журнала ошибок.
// Возвращает ID записей, которые были удалены.
func (re *CrashedPostgres) Delete(ctx context.Context, expiry time.Duration, limit int) (rowsIDs []uint64, err error) {
	sql := `
		WITH crashed_expired_items as (
			SELECT
			  	` + re.table.PrimaryKey + ` as item_id
			FROM
			  	` + re.table.Name + `
			GROUP BY
				item_id
			HAVING
				MAX(created_at) <= NOW() - INTERVAL '1 second' * $1
			ORDER BY
				MAX(created_at) ASC
		    LIMIT $2
		)
		DELETE FROM
			` + re.table.Name + ` t1
		USING
			crashed_expired_items bei
		WHERE
			t1.` + re.table.PrimaryKey + ` = bei.item_id
		RETURNING
			bei.item_id;`

	return fetchRowsIDs(
		ctx,
		re.client,
		sql,
		limit,
		uint32(expiry.Seconds()),
		limit,
	)
}
