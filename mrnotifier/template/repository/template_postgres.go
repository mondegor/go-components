package repository

import (
	"context"
	"fmt"

	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrmsg"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrenum"
	"github.com/mondegor/go-webcore/mrlog"

	"github.com/mondegor/go-components/mrnotifier/template/entity"
)

type (
	// TemplatePostgres - репозиторий для хранения шаблонов уведомлений.
	TemplatePostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper mrcore.StorageErrorWrapper
		storageVar   *VariablePostgres
		tableName    string
	}
)

// NewTemplatePostgres - создаёт объект TemplatePostgres.
func NewTemplatePostgres(client mrstorage.DBConnManager, tableName, tableVarsName string, errorWrapper mrcore.StorageErrorWrapper) *TemplatePostgres {
	return &TemplatePostgres{
		client:       client,
		errorWrapper: errorWrapper,
		storageVar:   NewVariablePostgres(client, tableVarsName, errorWrapper),
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
		return entity.Template{}, re.errorWrapper.WrapErrorEntity(err, re.tableName, mrmsg.Data{"name": name, "lang": lang})
	}

	if status != mrenum.ItemStatusEnabled {
		return entity.Template{}, re.errorWrapper.WrapErrorEntity(
			mrcore.ErrStorageNoRowFound.Wrap(fmt.Errorf("%s is in status %s, expected: %s", entity.ModelNameTemplate, status, mrenum.ItemStatusEnabled)),
			re.tableName,
			mrmsg.Data{"name": name, "lang": lang},
		)
	}

	if len(vars) > 0 {
		varRows, err := re.storageVar.Fetch(ctx, vars)
		if err != nil {
			return entity.Template{}, re.errorWrapper.WrapErrorEntity(err, re.tableName, mrmsg.Data{"name": name, "lang": lang})
		}

		if len(vars) != len(varRows) {
			mrlog.Ctx(ctx).
				Warn().
				Str("source", re.tableName).
				Str("entity", name+"+"+lang).
				Msgf("vars count: %d, expected: %d", len(varRows), len(vars))
		}

		row.Vars = varRows
	}

	return row, nil
}
