package repository

import (
	"context"

	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/mrstorage/mrsql"

	"github.com/mondegor/go-components/mrmailer/dto"
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

	ids := make([]uint64, 0, len(rows))
	channels := make([]string, 0, len(rows))
	datas := make([]dto.MessageData, 0, len(rows))

	for _, row := range rows {
		ids = append(ids, row.ID)
		channels = append(channels, row.Channel)
		datas = append(datas, row.Data)
	}

	sql := `
		INSERT INTO ` + re.table.Name + `
			(
				` + re.table.PrimaryKey + `,
				message_channel,
				message_data
			)
		SELECT *
		FROM
			UNNEST($1::int8[], $2::text[], $3::jsonb[])
			as t(id, message_channel, message_data);`

	return re.client.Conn(ctx).Exec(
		ctx,
		sql,
		ids,
		channels,
		datas,
	)
}

// DeleteByIDs - удаляет сообщения по их указанным ID.
func (re *MessagePostgres) DeleteByIDs(ctx context.Context, rowsIDs []uint64) error {
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
