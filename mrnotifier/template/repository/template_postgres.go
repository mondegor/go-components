package repository

import (
	"context"
	"fmt"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrargs"
	"github.com/mondegor/go-sysmess/mrerr/mr"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-webcore/mrenum"

	core "github.com/mondegor/go-components/internal"
	"github.com/mondegor/go-components/mrnotifier/template/entity"
)

type (
	// TemplatePostgres - репозиторий для хранения шаблонов уведомлений.
	TemplatePostgres struct {
		client       mrstorage.DBConnManager
		storageVar   *VariablePostgres
		tableName    string
		logger       mrlog.Logger
		errorWrapper core.ErrorWrapper
	}
)

// NewTemplatePostgres - создаёт объект TemplatePostgres.
func NewTemplatePostgres(client mrstorage.DBConnManager, tableName, tableVarsName string, logger mrlog.Logger) *TemplatePostgres {
	return &TemplatePostgres{
		client:       client,
		logger:       logger,
		storageVar:   NewVariablePostgres(client, tableVarsName),
		tableName:    tableName,
		errorWrapper: core.NewStorageErrorWrapper(tableName),
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
		status mrenum.ItemStatus
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

	if status != mrenum.ItemStatusEnabled {
		return entity.Template{}, mr.ErrStorageNoRowFound.Wrap(
			fmt.Errorf("model is in status %s, expected: %s", status, mrenum.ItemStatusEnabled),
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
