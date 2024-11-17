package repository

import (
	"context"

	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"

	"github.com/mondegor/go-components/mrnotifier/notifier/entity"
)

type (
	// NoticePostgres - репозиторий для хранения уведомлений.
	NoticePostgres struct {
		client mrstorage.DBConnManager
		table  mrsql.DBTableInfo
	}
)

// NewNoticePostgres - создаёт объект NoticePostgres.
func NewNoticePostgres(client mrstorage.DBConnManager, table mrsql.DBTableInfo) *NoticePostgres {
	return &NoticePostgres{
		client: client,
		table:  table,
	}
}

// FetchByIDs - возвращает список уведомлений по их указанным ID.
func (re *NoticePostgres) FetchByIDs(ctx context.Context, rowsIDs []uint64) ([]entity.Notice, error) {
	sql := `
		SELECT
			` + re.table.PrimaryKey + `,
			notice_key,
			notice_data
		FROM
			` + re.table.Name + `
		WHERE
			` + re.table.PrimaryKey + ` = ANY($1);`

	cursor, err := re.client.Conn(ctx).Query(
		ctx,
		sql,
		rowsIDs,
	)
	if err != nil {
		return nil, err
	}

	defer cursor.Close()

	rows := make([]entity.Notice, 0, len(rowsIDs))

	for cursor.Next() {
		var row entity.Notice

		err = cursor.Scan(
			&row.ID,
			&row.Key,
			&row.Data,
		)
		if err != nil {
			return nil, err
		}

		rows = append(rows, row)
	}

	return rows, cursor.Err()
}

// Insert - вставляет новое уведомление.
func (re *NoticePostgres) Insert(ctx context.Context, row entity.Notice) error {
	sql := `
		INSERT INTO ` + re.table.Name + `
			(
				` + re.table.PrimaryKey + `,
				notice_key,
				notice_data,
				created_at
			)
		VALUES
			($1, $2, $3, NOW());`

	return re.client.Conn(ctx).Exec(
		ctx,
		sql,
		row.ID,
		row.Key,
		row.Data,
	)
}

// DeleteByIDs - удаляет уведомления по их указанным ID.
func (re *NoticePostgres) DeleteByIDs(ctx context.Context, rowsIDs []uint64) error {
	sql := `
		DELETE FROM
			` + re.table.Name + `
		WHERE
			` + re.table.PrimaryKey + ` = ANY($1);`

	return re.client.Conn(ctx).Exec(
		ctx,
		sql,
		rowsIDs,
	)
}
