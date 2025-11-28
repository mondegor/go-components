package repository

import (
	"context"
	"fmt"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrargs"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrstatus/itemstatus"

	"github.com/mondegor/go-components/mrnotifier/template/entity"
)

type (
	// TemplatePostgres - репозиторий для хранения шаблонов уведомлений.
	TemplatePostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper mrerr.ErrorWrapper
		logger       mrlog.Logger
		storageVar   *VariablePostgres
		tableName    string
	}
)

// NewTemplatePostgres - создаёт объект TemplatePostgres.
func NewTemplatePostgres(
	client mrstorage.DBConnManager,
	errorWrapper mrerr.ErrorWrapper,
	logger mrlog.Logger,
	tableName string,
	tableVarsName string,
) *TemplatePostgres {
	errorWrapper = mrerr.NewErrorWrapper(errorWrapper, tableName)

	return &TemplatePostgres{
		client:       client,
		errorWrapper: errorWrapper,
		logger:       logger,
		storageVar:   NewVariablePostgres(client, errorWrapper, tableVarsName),
		tableName:    tableName,
	}
}

// FetchOneByKey - возвращает шаблон уведомления по указанному имени (ключу) и языку.
func (re *TemplatePostgres) FetchOneByKey(ctx context.Context, name, lang string) (row entity.Template, err error) {
	sql := `
		SELECT
			lang_code,
			notice_props,
			notice_vars,
			template_status
		FROM
			` + re.tableName + `
		WHERE
			template_name = $1 AND lang_code = $2 AND deleted_at IS NULL
		LIMIT 1;`

	var (
		status itemstatus.Enum
		vars   []string
	)

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		name,
		lang,
	).Scan(
		&row.Lang,
		&row.Props,
		&vars,
		&status,
	)
	if err != nil {
		return entity.Template{}, re.errorWrapper.WrapError(err, "storage-data", mrargs.Group{"name": name, "lang": lang})
	}

	if status != itemstatus.Enabled {
		return entity.Template{}, mr.ErrStorageNoRowFound.Wrap(
			fmt.Errorf("model is in status '%s', expected: '%s'", status, itemstatus.Enabled),
			"storage-data", mrargs.Group{"name": name, "lang": lang},
		)
	}

	if len(vars) > 0 {
		varRows, err := re.storageVar.Fetch(ctx, vars)
		if err != nil {
			return entity.Template{}, re.errorWrapper.WrapError(err, "storage-data", mrargs.Group{"name": name, "lang": lang})
		}

		if len(vars) != len(varRows) {
			re.logger.Warn(
				ctx,
				fmt.Sprintf("vars count: %d, expected: %d", len(varRows), len(vars)),
				"source", re.tableName,
				"entity", name+"+"+lang,
			)
		}

		row.Vars = varRows
	}

	return row, nil
}
