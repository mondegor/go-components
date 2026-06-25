package repository

import (
	"context"

	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-sysmess/mrstorage/mrsql"

	"github.com/mondegor/go-components/mrnotifier/notifier/entity"
)

type (
	// NotePostgres - репозиторий для хранения уведомлений.
	NotePostgres struct {
		client mrstorage.DBConnManager
		table  mrsql.DBTableInfo
	}
)

// NewNotePostgres - создаёт объект NotePostgres.
func NewNotePostgres(
	client mrstorage.DBConnManager,
	table mrsql.DBTableInfo,
) *NotePostgres {
	return &NotePostgres{
		client: client,
		table:  table,
	}
}

// FetchByIDs - возвращает список уведомлений по их указанным ID.
func (re *NotePostgres) FetchByIDs(ctx context.Context, rowsIDs []uint64) ([]entity.Note, error) {
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

	rows := make([]entity.Note, 0, len(rowsIDs))

	for cursor.Next() {
		var row entity.Note

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
func (re *NotePostgres) Insert(ctx context.Context, row entity.Note) error {
	sql := `
		INSERT INTO ` + re.table.Name + `
			(
				` + re.table.PrimaryKey + `,
				notice_key,
				notice_data
			)
		VALUES
			($1, $2, $3);`

	return re.client.Conn(ctx).Exec(
		ctx,
		sql,
		row.ID,
		row.Key,
		row.Data,
	)
}

// DeleteByIDs - удаляет уведомления по их указанным ID.
func (re *NotePostgres) DeleteByIDs(ctx context.Context, rowsIDs []uint64) error {
	sql := `
		DELETE FROM
			` + re.table.Name + `
		WHERE
			` + re.table.PrimaryKey + ` = ANY($1);`

	_, err := re.client.Conn(ctx).ExecAffected(
		ctx,
		sql,
		rowsIDs,
	)
	if err != nil {
		return err // TODO: errorWrapper
	}

	return nil
}
