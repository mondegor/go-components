package repository

import (
	"context"
	"time"

	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
)

type (
	// CompletedPostgres - репозиторий для хранения успешно обработанных записей.
	CompletedPostgres struct {
		client mrstorage.DBConnManager
		table  mrsql.DBTableInfo
	}
)

// NewCompletedPostgres - создаёт объект CompletedPostgres.
func NewCompletedPostgres(client mrstorage.DBConnManager, table mrsql.DBTableInfo) *CompletedPostgres {
	return &CompletedPostgres{
		client: client,
		table:  table,
	}
}

// Insert - добавляет указанную запись в список успешно обработанных.
func (re *CompletedPostgres) Insert(ctx context.Context, rowID uint64) error {
	sql := `
		INSERT INTO ` + re.table.Name + `
			(
				` + re.table.PrimaryKey + `,
				updated_at
			)
		VALUES
			($1, NOW());`

	return re.client.Conn(ctx).Exec(
		ctx,
		sql,
		rowID,
	)
}

// Delete - удаляет ограниченный список записей из успешно обработанных.
// Возвращает ID записей, которые были удалены.
func (re *CompletedPostgres) Delete(ctx context.Context, expiry time.Duration, limit uint32) (rowsIDs []uint64, err error) {
	sql := `
		WITH completed_expired_items as (
			SELECT
			  	` + re.table.PrimaryKey + ` as item_id
			FROM
			  	` + re.table.Name + `
			WHERE
				updated_at <= NOW() - INTERVAL '1 second' * $1
			ORDER BY
				updated_at ASC
		    LIMIT $2
		)
		DELETE FROM
			` + re.table.Name + ` t1
		USING
			completed_expired_items cei
		WHERE
			t1.` + re.table.PrimaryKey + ` = cei.item_id
		RETURNING
			cei.item_id;`

	return fetchRowsIDs(
		ctx,
		re.client,
		sql,
		limit,
		uint32(expiry.Seconds()),
		limit,
	)
}
