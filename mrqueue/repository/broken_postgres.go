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
	// BrokenPostgres - репозиторий для хранения ошибок записей, которые случились при их обработке.
	BrokenPostgres struct {
		client mrstorage.DBConnManager
		table  mrsql.DBTableInfo
	}
)

// NewBrokenPostgres - создаёт объект BrokenPostgres.
func NewBrokenPostgres(client mrstorage.DBConnManager, table mrsql.DBTableInfo) *BrokenPostgres {
	return &BrokenPostgres{
		client: client,
		table:  table,
	}
}

// Insert - добавляет указанный список записей в журнал ошибок.
func (re *BrokenPostgres) Insert(ctx context.Context, rows []entity.ItemWithError) error {
	if len(rows) == 0 {
		return nil
	}

	var sql strings.Builder

	sql.WriteString(`
		INSERT INTO ` + re.table.Name + `
			(
				` + re.table.PrimaryKey + `,
				error_message,
				created_at
			)
		VALUES `)

	const countLineArgs = 2

	// generate: ($1, $2, NOW()), ...
	sqlValues := placeholdedvalues.New(
		&sql,
		placeholdedvalues.WithCountArgs(countLineArgs),
		placeholdedvalues.WithLinePostfix(", NOW()"),
	)

	values := make([]any, 0, len(rows)*countLineArgs)

	argumentNumber := sqlValues.WriteFirstLine()

	for i, row := range rows {
		if i > 0 {
			argumentNumber = sqlValues.WriteNextLine(argumentNumber)
		}

		values = append(values, row.ID, row.Err.Error())
	}

	sql.WriteByte(';')

	return re.client.Conn(ctx).Exec(
		ctx,
		sql.String(),
		values...,
	)
}

// InsertOne - добавляет указанную запись в журнал ошибок.
func (re *BrokenPostgres) InsertOne(ctx context.Context, row entity.ItemWithError) error {
	return re.Insert(ctx, []entity.ItemWithError{row})
}

// Delete - удаляет ограниченный список записей из журнала ошибок.
// Возвращает ID записей, которые были удалены.
func (re *BrokenPostgres) Delete(ctx context.Context, expiry time.Duration, limit uint32) (rowsIDs []uint64, err error) {
	sql := `
		WITH broken_expired_items as (
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
			broken_expired_items bei
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
