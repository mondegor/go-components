package repository

import (
	"context"
	"strings"
	"time"

	"github.com/mondegor/go-storage/mrpostgres/stream/placeholdedvalues"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrerr"

	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// SecureOperationLogPostgres - репозиторий для хранения элементов настроек.
	SecureOperationLogPostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper mrerr.ErrorWrapper
		tableName    string
	}
)

// NewSecureOperationLogPostgres - создаёт объект SecureOperationLogPostgres.
func NewSecureOperationLogPostgres(
	client mrstorage.DBConnManager,
	errorWrapper mrerr.ErrorWrapper,
	tableName string,
) *SecureOperationLogPostgres {
	return &SecureOperationLogPostgres{
		client:       client,
		errorWrapper: mrerr.NewErrorWrapper(errorWrapper, tableName),
		tableName:    tableName,
	}
}

// Insert - фиксирует изменение настройки.
func (re *SecureOperationLogPostgres) Insert(ctx context.Context, rows []entity.SecureOperationLog) error {
	if len(rows) == 0 {
		return nil
	}

	var sql strings.Builder

	sql.WriteString(`
		INSERT INTO ` + re.tableName + `
			(
				visitor_id,
				operation_name,
				confirm_method,
				log_status
			)
		VALUES `)

	const countLineArgs = 6

	// generate: ($1, $2, $3, $4), ...
	sqlValues := placeholdedvalues.New(
		&sql,
		placeholdedvalues.WithCountArgs(countLineArgs),
	)

	values := make([]any, 0, len(rows)*countLineArgs)
	argumentNumber := sqlValues.WriteFirstLine()

	for i, row := range rows {
		if i > 0 {
			argumentNumber = sqlValues.WriteNextLine(argumentNumber)
		}

		values = append(
			values,
			row.VisitorID, row.OperationName, row.ConfirmMethod, row.LogStatus,
		)
	}

	sql.WriteByte(';')

	return re.client.Conn(ctx).Exec(
		ctx,
		sql.String(),
		values...,
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
	if err != nil {
		return re.errorWrapper.WrapError(err)
	}

	return nil
}
