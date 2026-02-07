package repository

import (
	"context"
	"strings"

	"github.com/mondegor/go-storage/mrpostgres/stream/placeholdedvalues"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrmailer/entity"
)

type (
	// MessagePostgres - репозиторий для хранения сообщений подготовленных для отправки различным получателям.
	MessagePostgres struct {
		client           mrstorage.DBConnManager
		table            mrsql.DBTableInfo
		insertArgsHelper placeholdedvalues.SQL
	}
)

// NewMessagePostgres - создаёт объект MessagePostgres.
func NewMessagePostgres(client mrstorage.DBConnManager, table mrsql.DBTableInfo) *MessagePostgres {
	const countLineArgs = 3

	return &MessagePostgres{
		client: client,
		table:  table,

		insertArgsHelper: placeholdedvalues.New(
			placeholdedvalues.WithCountLineArgs(countLineArgs),
		),
	}
}

// FetchByIDs - возвращает список сообщений по их указанным ID.
func (re *MessagePostgres) FetchByIDs(ctx context.Context, rowsIDs []uint64) ([]entity.Message, error) {
	sql := `
		SELECT
			` + re.table.PrimaryKey + `,
			message_channel,
			message_data
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

	rows := make([]entity.Message, 0, len(rowsIDs))

	for cursor.Next() {
		var row entity.Message

		err = cursor.Scan(
			&row.ID,
			&row.Channel,
			&row.Data,
		)
		if err != nil {
			return nil, err
		}

		rows = append(rows, row)
	}

	return rows, cursor.Err()
}

// Insert - вставляет новое сообщение.
func (re *MessagePostgres) Insert(ctx context.Context, rows []entity.Message) error {
	if len(rows) == 0 {
		return nil
	}

	var sql strings.Builder

	sql.WriteString(`
		INSERT INTO ` + re.table.Name + `
			(
				` + re.table.PrimaryKey + `,
				message_channel,
				message_data
			)
		VALUES `)

	// generate: ($1, $2, $3), ...
	values := make([]any, 0, len(rows)*re.insertArgsHelper.CountLineArgs())
	argumentNumber := re.insertArgsHelper.WriteFirstLine(&sql)

	for i, row := range rows {
		if i > 0 {
			argumentNumber = re.insertArgsHelper.WriteNextLine(&sql, argumentNumber)
		}

		values = append(values, row.ID, row.Channel, row.Data)
	}

	sql.WriteByte(';')

	return re.client.Conn(ctx).Exec(
		ctx,
		sql.String(),
		values...,
	)
}

// DeleteByIDs - удаляет сообщения по их указанным ID.
func (re *MessagePostgres) DeleteByIDs(ctx context.Context, rowsIDs []uint64) error {
	sql := `
		DELETE FROM
			` + re.table.Name + `
		WHERE
			` + re.table.PrimaryKey + ` = ANY($1);`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		rowsIDs,
	)
	// если это внутренняя ошибка
	if err != nil && !errors.Is(err, errors.ErrEventStorageRowsNotAffected) {
		return err
	}

	return nil
}
