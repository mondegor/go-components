package repository

import (
	"context"
	"time"

	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/mrstorage/mrsql"

	"github.com/mondegor/go-components/mrqueue/entity"
)

type (
	// CrashedPostgres - репозиторий для хранения ошибок записей, которые случились при их обработке.
	CrashedPostgres struct {
		client mrstorage.DBConnManager
		table  mrsql.DBTableInfo
	}
)

// NewCrashedPostgres - создаёт объект CrashedPostgres.
func NewCrashedPostgres(client mrstorage.DBConnManager, table mrsql.DBTableInfo) *CrashedPostgres {
	return &CrashedPostgres{
		client: client,
		table:  table,
	}
}

// Insert - добавляет указанный список записей в журнал ошибок.
func (re *CrashedPostgres) Insert(ctx context.Context, rows []entity.CrashedItem) error {
	if len(rows) == 0 {
		return nil
	}

	ids := make([]uint64, 0, len(rows))
	causes := make([]string, 0, len(rows))

	for _, row := range rows {
		ids = append(ids, row.ID)
		causes = append(causes, row.Cause)
	}

	sql := `
		INSERT INTO ` + re.table.Name + `
			(
				` + re.table.PrimaryKey + `,
				error_message
			)
		SELECT *
		FROM
			UNNEST($1::int8[], $2::text[])
			as t(id, error_message);`

	return re.client.Conn(ctx).Exec(
		ctx,
		sql,
		ids,
		causes,
	)
}

// InsertOne - добавляет указанную запись в журнал ошибок.
func (re *CrashedPostgres) InsertOne(ctx context.Context, row entity.CrashedItem) error {
	return re.Insert(ctx, []entity.CrashedItem{row})
}

// Delete - удаляет из журнала ошибок все ошибки ограниченного списка записей,
// последняя ошибка которых старше expiry. Возвращает ID этих записей.
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
		    ` + mrstorage.NonZeroLimit(limit) + `
		),
		deleted_items as (
			DELETE FROM
				` + re.table.Name + ` t1
			USING
				crashed_expired_items bei
			WHERE
				t1.` + re.table.PrimaryKey + ` = bei.item_id
			RETURNING
				bei.item_id
		)
		SELECT DISTINCT
			item_id
		FROM
			deleted_items;`

	return fetchRowsIDs(
		ctx,
		re.client,
		sql,
		limit,
		uint32(expiry.Seconds()),
	)
}
