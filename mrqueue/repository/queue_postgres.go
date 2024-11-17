package repository

import (
	"context"
	"strings"
	"time"

	"github.com/mondegor/go-storage/mrpostgres/stream/placeholdedvalues"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"

	"github.com/mondegor/go-components/mrqueue/entity"
	"github.com/mondegor/go-components/mrqueue/enum"
)

type (
	// QueuePostgres - репозиторий для организации очереди и хранения в ней записей.
	QueuePostgres struct {
		client mrstorage.DBConnManager
		table  mrsql.DBTableInfo
	}
)

// NewQueuePostgres - создаёт объект QueuePostgres.
func NewQueuePostgres(client mrstorage.DBConnManager, table mrsql.DBTableInfo) *QueuePostgres {
	return &QueuePostgres{
		client: client,
		table:  table,
	}
}

// Insert - добавляет список записей в очередь со статусом READY.
// Если указано ReadyDelayed, то обработка записи откладывается на указанный период времени.
func (re *QueuePostgres) Insert(ctx context.Context, rows []entity.Item) error {
	if len(rows) == 0 {
		return nil
	}

	var sql strings.Builder

	sql.WriteString(`
		INSERT INTO ` + re.table.Name + `
			(
				` + re.table.PrimaryKey + `,
				remaining_attempts,
				item_status,
				updated_at
			)
		VALUES `)

	const countLineArgs = 4

	// generate: ($1, $2, $3, NOW() + INTERVAL '1 second' * $4), ...
	sqlValues := placeholdedvalues.New(
		&sql,
		placeholdedvalues.WithCountArgs(countLineArgs),
		placeholdedvalues.WithLineMiddle(map[uint32]string{countLineArgs - 1: ", NOW() + INTERVAL '1 second' * "}),
	)

	values := make([]any, 0, len(rows)*countLineArgs)
	argumentNumber := sqlValues.WriteFirstLine()

	for i, row := range rows {
		if i > 0 {
			argumentNumber = sqlValues.WriteNextLine(argumentNumber)
		}

		values = append(values, row.ID, row.RetryAttempts, enum.ItemStatusReady, row.ReadyDelayed.Seconds())
	}

	sql.WriteByte(';')

	return re.client.Conn(ctx).Exec(
		ctx,
		sql.String(),
		values...,
	)
}

// FetchAndUpdateStatusReadyToProcessing - выбирает ограниченный список записей из очереди находящихся в статусе READY
// в порядке их добавления и переводит эти записи в статус PROCESSING.
func (re *QueuePostgres) FetchAndUpdateStatusReadyToProcessing(ctx context.Context, limit uint32) (rowsIDs []uint64, err error) {
	sql := `
		WITH ready_to_processing as (
			SELECT
			  	` + re.table.PrimaryKey + ` as item_id
			FROM
			  	` + re.table.Name + `
			WHERE
			  	item_status = $1 AND updated_at <= NOW()
			ORDER BY
				updated_at ASC
		    LIMIT $3
			FOR UPDATE SKIP LOCKED
		)
		UPDATE
			` + re.table.Name + ` t1
		SET
			item_status = $2,
			updated_at = NOW()
	   	FROM
			ready_to_processing rtp
		WHERE
			t1.` + re.table.PrimaryKey + ` = rtp.item_id
		RETURNING
			rtp.item_id;`

	return fetchRowsIDs(
		ctx,
		re.client,
		sql,
		limit,
		enum.ItemStatusReady,
		enum.ItemStatusProcessing,
		limit,
	)
}

// UpdateStatusProcessingToReady - возвращает указанные записи в статус READY, но только
// если они находятся в статусе PROCESSING (например, в случае отмены обработки этих записей).
func (re *QueuePostgres) UpdateStatusProcessingToReady(ctx context.Context, rowsIDs []uint64) error {
	sql := `
		UPDATE
			` + re.table.Name + `
		SET
			item_status = $3,
			updated_at = NOW()
		WHERE
			` + re.table.PrimaryKey + ` = ANY($1) AND item_status = $2;`

	return re.client.Conn(ctx).Exec(
		ctx,
		sql,
		rowsIDs,
		enum.ItemStatusProcessing,
		enum.ItemStatusReady,
	)
}

// UpdateStatusProcessingToRetry - переводит указанную запись из статуса PROCESSING в статус RETRY,
// с уменьшением кол-ва попыток (например, в случае возникновения ошибки при обработке этой записи).
func (re *QueuePostgres) UpdateStatusProcessingToRetry(ctx context.Context, rowID uint64) error {
	sql := `
		UPDATE
			` + re.table.Name + `
		SET
			item_status = $3,
			remaining_attempts = remaining_attempts - 1,
			updated_at = NOW()
		WHERE
			` + re.table.PrimaryKey + ` = $1 AND item_status = $2;`

	return re.client.Conn(ctx).Exec(
		ctx,
		sql,
		rowID,
		enum.ItemStatusProcessing,
		enum.ItemStatusRetry,
	)
}

// FetchAndUpdateStatusProcessingToRetryByTimeout - возвращает ограниченный список записей находящихся долгое время
// в статусе PROCESSING (например, в случае если обработка записи подвисла) предварительно переведя их в статус RETRY.
func (re *QueuePostgres) FetchAndUpdateStatusProcessingToRetryByTimeout(ctx context.Context, timeout time.Duration, limit uint32) (rowIDs []uint64, err error) {
	sql := `
		WITH processing_to_retry as (
			SELECT
			  	` + re.table.PrimaryKey + ` as item_id
			FROM
			  	` + re.table.Name + `
			WHERE
			  	item_status = $1 AND updated_at < NOW() - INTERVAL '1 second' * $2
			ORDER BY
				updated_at ASC
		    LIMIT $4
			FOR UPDATE SKIP LOCKED
		)
		UPDATE
			` + re.table.Name + ` t1
		SET
			item_status = $3,
			updated_at = NOW()
	   	FROM
			processing_to_retry ptr
		WHERE
			t1.` + re.table.PrimaryKey + ` = ptr.item_id
		RETURNING
			ptr.item_id;`

	return fetchRowsIDs(
		ctx,
		re.client,
		sql,
		limit,
		enum.ItemStatusProcessing,
		uint32(timeout.Seconds()),
		enum.ItemStatusRetry,
		limit,
	)
}

// FetchAndUpdateStatusRetryToReady - переводит ограниченный список записей из статуса RETRY в статус READY
// учитывая указанную задержку нахождения записи в этом статусе и положительное кол-во попыток.
func (re *QueuePostgres) FetchAndUpdateStatusRetryToReady(ctx context.Context, delayed time.Duration, limit uint32) (rowIDs []uint64, err error) {
	sql := `
		WITH retry_to_ready as (
			SELECT
			  	` + re.table.PrimaryKey + ` as item_id
			FROM
			  	` + re.table.Name + `
			WHERE
			  	item_status = $1 AND
				updated_at <= NOW() - INTERVAL '1 second' * $2 AND
				remaining_attempts > 0
			ORDER BY
				updated_at ASC
		    LIMIT $4
			FOR UPDATE SKIP LOCKED
		)
		UPDATE
			` + re.table.Name + `
		SET
			item_status = $3,
			updated_at = NOW()
	   	FROM
			retry_to_ready rtr
		WHERE
			` + re.table.PrimaryKey + ` = rtr.item_id
		RETURNING
			rtr.item_id;`

	return fetchRowsIDs(
		ctx,
		re.client,
		sql,
		limit,
		enum.ItemStatusRetry,
		uint32(delayed.Seconds()),
		enum.ItemStatusReady,
		limit,
	)
}

// DeleteRetryWithoutAttempts - удаляет из очереди ограниченный список записей находящихся
// в статусе RETRY и с нулевым кол-вом попыток в целях разгрузки очереди. Возвращает ID записей, которые были удалены.
func (re *QueuePostgres) DeleteRetryWithoutAttempts(ctx context.Context, limit uint32) (rowsIDs []uint64, err error) {
	sql := `
		WITH retry_without_attempts as (
			SELECT
			  	` + re.table.PrimaryKey + ` as item_id
			FROM
			  	` + re.table.Name + `
			WHERE
			  	item_status = $1 AND remaining_attempts = 0
			ORDER BY
				updated_at ASC
		    LIMIT $2
		)
		DELETE FROM
			` + re.table.Name + ` t1
		USING
			retry_without_attempts rwa
		WHERE
			t1.` + re.table.PrimaryKey + ` = rwa.item_id
		RETURNING
			rwa.item_id;`

	return fetchRowsIDs(
		ctx,
		re.client,
		sql,
		limit,
		enum.ItemStatusRetry,
		limit,
	)
}

// Delete - удаляет запись из очереди по указанному ID и находящеюся в указанном статусе.
func (re *QueuePostgres) Delete(ctx context.Context, rowID uint64, status enum.ItemStatus) error {
	sql := `
		DELETE FROM
			` + re.table.Name + `
		WHERE
			` + re.table.PrimaryKey + ` = $1 AND item_status = $2;`

	return re.client.Conn(ctx).Exec(
		ctx,
		sql,
		rowID,
		status,
	)
}
