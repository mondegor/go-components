package repository

import (
	"context"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/util/conv"

	"github.com/mondegor/go-components/mrnotifier/template/entity"
)

type (
	// VariablePostgres - репозиторий для хранения переменных шаблонов со значениями по умолчанию.
	VariablePostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper errors.Wrapper
		tableName    string
	}
)

// NewVariablePostgres - создаёт объект VariablePostgres.
func NewVariablePostgres(
	client mrstorage.DBConnManager,
	tableName string,
) *VariablePostgres {
	return &VariablePostgres{
		client:       client,
		tableName:    tableName,
		errorWrapper: errors.NewInfraStorageWrapper(),
	}
}

// Fetch - возвращает список переменных со значениями по умолчанию по их названиям.
func (re *VariablePostgres) Fetch(ctx context.Context, vars []string) (rows []entity.Variable, err error) {
	sql := `
		SELECT
			var_name,
			default_value
		FROM
			` + re.tableName + `
		WHERE
			var_name = ANY($1);`

	cursor, err := re.client.Conn(ctx).Query(
		ctx,
		sql,
		vars,
	)
	if err != nil {
		return nil, re.errorWrapper.Wrap(err, "log.storage_data", conv.Group{"vars": vars})
	}

	defer cursor.Close()

	rows = make([]entity.Variable, 0, len(vars))

	for cursor.Next() {
		var row entity.Variable

		err = cursor.Scan(
			&row.Name,
			&row.DefaultValue,
		)
		if err != nil {
			return nil, re.errorWrapper.Wrap(err, "log.storage_data", conv.Group{"vars": vars})
		}

		rows = append(rows, row)
	}

	return rows, cursor.Err()
}
