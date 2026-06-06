package repository

import (
	"context"

	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-sysmess/util/conv"

	"github.com/mondegor/go-components/mrnotifier/template/entity"
)

type (
	// TemplatePostgres - репозиторий для хранения шаблонов уведомлений.
	TemplatePostgres struct {
		client       mrstorage.DBConnManager
		errorWrapper errors.Wrapper
		tableName    string
	}
)

// NewTemplatePostgres - создаёт объект TemplatePostgres.
func NewTemplatePostgres(
	client mrstorage.DBConnManager,
	tableName string,
) *TemplatePostgres {
	return &TemplatePostgres{
		client:       client,
		errorWrapper: errors.NewInfraStorageWrapper(),
		tableName:    tableName,
	}
}

// FetchOneByKey - возвращает шаблон уведомления по указанному имени (ключу) и языку.
func (re *TemplatePostgres) FetchOneByKey(ctx context.Context, name, lang string) (row entity.Template, err error) {
	sql := `
		SELECT
			notice_props,
			notice_vars,
			template_status
		FROM
			` + re.tableName + `
		WHERE
			template_name = $1 AND lang_code = $2 AND deleted_at IS NULL
		LIMIT 1;`

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		name,
		lang,
	).Scan(
		&row.Props,
		&row.Vars,
		&row.Status,
	)
	if err != nil {
		return entity.Template{}, re.errorWrapper.Wrap(err, "log.storage_data", conv.Group{"name": name, "lang": lang})
	}

	row.Name = name
	row.Lang = lang

	return row, nil
}
