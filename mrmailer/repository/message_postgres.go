package repository

import (
	"context"
	"strings"

	"github.com/mondegor/go-storage/mrpostgres/stream/placeholdedvalues"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"

	"github.com/mondegor/go-components/mrmailer/entity"
)

type (
	// MessagePostgres - репозиторий для хранения сообщений подготовленных для отправки различным получателям.
	MessagePostgres struct {
		client mrstorage.DBConnManager
		table  mrsql.DBTableInfo
	}
)

// NewMessagePostgres - создаёт объект MessagePostgres.
func NewMessagePostgres(client mrstorage.DBConnManager, table mrsql.DBTableInfo) *MessagePostgres {
	return &MessagePostgres{
		client: client,
		table:  table,
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
				message_data,
				created_at
			)
		VALUES `)

	const countLineArgs = 3

	// generate: ($1, $2, $3, NOW()), ...
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

	return re.client.Conn(ctx).Exec(
		ctx,
		sql,
		rowsIDs,
	)
}
