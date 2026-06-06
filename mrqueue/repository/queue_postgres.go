package repository

import (
	"context"
	"time"

	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-sysmess/mrstorage/mrsql"

	"github.com/mondegor/go-components/mrqueue/dto"
	"github.com/mondegor/go-components/mrqueue/enum/itemstatus"
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
func (re *QueuePostgres) Insert(ctx context.Context, rows []dto.Item) error {
	if len(rows) == 0 {
		return nil
	}

	ids := make([]uint64, 0, len(rows))
	retryAttempts := make([]int16, 0, len(rows))
	readyDelayed := make([]int32, 0, len(rows))

	for _, row := range rows {
		ids = append(ids, row.ID)
		retryAttempts = append(retryAttempts, row.RetryAttempts)
		readyDelayed = append(readyDelayed, int32(row.ReadyDelayed.Milliseconds()/1000)) //nolint:gosec
	}

	sql := `
		INSERT INTO ` + re.table.Name + `
			(
				` + re.table.PrimaryKey + `,
				remaining_attempts,
				item_status,
				updated_at
			)
		SELECT id, remaining_attempts, $4, NOW() + INTERVAL '1 second' * ready_delayed
		FROM
			UNNEST($1::int8[], $2::int2[], $3::int4[])
			as t(id, remaining_attempts, ready_delayed);`

	return re.client.Conn(ctx).Exec(
		ctx,
		sql,
		ids,
		retryAttempts,
		readyDelayed,
		itemstatus.Ready,
	)
}

// FetchAndUpdateStatusReadyToProcessing - выбирает ограниченный список записей из очереди находящихся в статусе READY
// в порядке их добавления и переводит эти записи в статус PROCESSING.
func (re *QueuePostgres) FetchAndUpdateStatusReadyToProcessing(ctx context.Context, limit int) (rowsIDs []uint64, err error) {
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
		itemstatus.Ready,
		itemstatus.Processing,
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
		itemstatus.Processing,
		itemstatus.Ready,
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

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		rowID,
		itemstatus.Processing,
		itemstatus.Retry,
	)
	if err != nil && errors.Is(err, errors.ErrEventStorageRecordsNotAffected) {
		return errors.ErrEventStorageNoRecordFound
	}

	return err
}

// UpdateStatusProcessingToRetryByTimeout - переводит ограниченный список записей из статуса PROCESSING в статус RETRY находящихся там долгое время
// (например, в случае если обработка записи подвисла).
func (re *QueuePostgres) UpdateStatusProcessingToRetryByTimeout(ctx context.Context, timeout time.Duration, limit int) (rowIDs []uint64, err error) {
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
		itemstatus.Processing,
		uint32(timeout.Seconds()),
		itemstatus.Retry,
		limit,
	)
}

// UpdateStatusRetryToReady - переводит ограниченный список записей из статуса RETRY в статус READY
// учитывая указанную задержку нахождения записи в этом статусе и положительное кол-во попыток.
func (re *QueuePostgres) UpdateStatusRetryToReady(ctx context.Context, delayed time.Duration, limit int) (rowIDs []uint64, err error) {
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
		itemstatus.Retry,
		uint32(delayed.Seconds()),
		itemstatus.Ready,
		limit,
	)
}

// DeleteRetryWithoutAttempts - удаляет из очереди ограниченный список записей находящихся
// в статусе RETRY и с нулевым кол-вом попыток в целях разгрузки очереди. Возвращает ID записей, которые были удалены.
func (re *QueuePostgres) DeleteRetryWithoutAttempts(ctx context.Context, limit int) (rowsIDs []uint64, err error) {
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
		itemstatus.Retry,
		limit,
	)
}

// Delete - удаляет запись из очереди по указанному rowID и находящеюся в указанном статусе.
func (re *QueuePostgres) Delete(ctx context.Context, rowID uint64, status itemstatus.Enum) error {
	sql := `
		DELETE FROM
			` + re.table.Name + `
		WHERE
			` + re.table.PrimaryKey + ` = $1 AND item_status = $2;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		rowID,
		status,
	)
	if err != nil && errors.Is(err, errors.ErrEventStorageRecordsNotAffected) {
		return errors.ErrEventStorageNoRecordFound
	}

	return err
}
