package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
)

type (
	// SecureOperationLogPostgres - репозиторий для хранения элементов настроек.
	SecureOperationLogPostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper errors.Wrapper
		tableName    string
	}
)

// NewSecureOperationLogPostgres - создаёт объект SecureOperationLogPostgres.
func NewSecureOperationLogPostgres(
	client mrstorage.DBConnManager,
	tableName string,
) *SecureOperationLogPostgres {
	return &SecureOperationLogPostgres{
		client:       client,
		errorWrapper: errors.NewInfraStorageWrapper(),
		tableName:    tableName,
	}
}

// Insert - фиксирует изменение настройки.
func (re *SecureOperationLogPostgres) Insert(ctx context.Context, rows []entity.SecureOperationLog) error {
	if len(rows) == 0 {
		return nil
	}

	ids := make([]uuid.UUID, 0, len(rows))
	operations := make([]string, 0, len(rows))
	methods := make([]confirmmethod.Enum, 0, len(rows))
	statuses := make([]string, 0, len(rows))

	for _, row := range rows {
		ids = append(ids, row.VisitorID)
		operations = append(operations, row.OperationName)
		methods = append(methods, row.ConfirmMethod)
		statuses = append(statuses, row.LogStatus)
	}

	sql := `
		INSERT INTO ` + re.tableName + `
			(
				visitor_id,
				operation_name,
				confirm_method,
				log_status
			)
		SELECT *
		FROM
			UNNEST($1::int8[], $2::text[], $3::int2, $4::text[])
			as t(visitor_id, operation_name, confirm_method, log_status);`

	return re.client.Conn(ctx).Exec(
		ctx,
		sql,
		ids,
		operations,
		methods,
		statuses,
	)
}

// DeleteBeforeDate - comments method.
func (re *SecureOperationLogPostgres) DeleteBeforeDate(ctx context.Context, datetime time.Time, limit int) error {
	sql := `
		WITH old_items as (
			SELECT
			  	record_id as item_id
			FROM
			  	` + re.tableName + `
			WHERE
				created_at < $1
			ORDER BY
				created_at ASC
		    LIMIT $2
		)
		DELETE FROM
			` + re.tableName + ` t1
		USING
			old_items ei
		WHERE
			t1.record_id = ei.item_id;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		datetime,
		limit,
	)
	// если это внутренняя ошибка
	if err != nil && !errors.Is(err, errors.ErrEventStorageRecordsNotAffected) {
		return re.errorWrapper.Wrap(err)
	}

	return nil
}
