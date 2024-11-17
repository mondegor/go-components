package repository

import (
	"context"
	"time"

	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrmsg"
	"github.com/mondegor/go-webcore/mrcore"

	"github.com/mondegor/go-components/mrsettings/entity"
)

const (
	defaultTableName  = "settings"
	defaultPrimaryKey = "setting_id"
)

type (
	// SettingPostgres - репозиторий для хранения элементов настроек.
	SettingPostgres struct {
		client       mrstorage.DBConnManager
		table        mrsql.DBTableInfo
		condBuilder  mrstorage.SQLConditionBuilder
		errorWrapper mrcore.StorageErrorWrapper
		condition    mrstorage.SQLPartFunc // OPTIONAL
	}
)

// NewSettingPostgres - создаёт объект SettingPostgres.
func NewSettingPostgres(
	client mrstorage.DBConnManager,
	table mrsql.DBTableInfo,
	whereBuilder mrstorage.SQLConditionBuilder,
	errorWrapper mrcore.StorageErrorWrapper,
	condition mrstorage.SQLPartFunc, // OPTIONAL
) *SettingPostgres {
	if table.Name == "" {
		table.Name = defaultTableName
	}

	if table.PrimaryKey == "" {
		table.PrimaryKey = defaultPrimaryKey
	}

	return &SettingPostgres{
		client:       client,
		errorWrapper: errorWrapper,
		table:        table,
		condBuilder:  whereBuilder,
		condition:    condition,
	}
}

// Fetch - возвращает список всех настроек. При использовании lastUpdated
// вернутся только те настройки, которые были обновлены не ранее указанной даты.
func (re *SettingPostgres) Fetch(ctx context.Context, lastUpdated time.Time) ([]entity.Setting, error) {
	whereStr, whereArgs := re.condBuilder.BuildFunc(
		func(w mrstorage.SQLConditionHelper) mrstorage.SQLPartFunc {
			return w.JoinAnd(
				re.condition,
				w.Greater("updated_at", lastUpdated),
			)
		},
	).ToSQL()

	sql := `
		SELECT
			` + re.table.PrimaryKey + `,
			setting_name,
			setting_type,
			setting_value,
			setting_description,
			updated_at
		FROM
			` + re.table.Name + `
		WHERE
			` + whereStr + `;`

	cursor, err := re.client.Conn(ctx).Query(
		ctx,
		sql,
		whereArgs...,
	)
	if err != nil {
		return nil, err
	}

	defer cursor.Close()

	rows := make([]entity.Setting, 0)

	for cursor.Next() {
		var row entity.Setting

		err = cursor.Scan(
			&row.ID,
			&row.Name,
			&row.Type,
			&row.Value,
			&row.Description,
			&row.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		rows = append(rows, row)
	}

	return rows, cursor.Err()
}

// FetchOne - возвращает настройку по указанному ID.
func (re *SettingPostgres) FetchOne(ctx context.Context, id uint64) (entity.Setting, error) {
	args := []any{
		id,
	}

	whereStr, whereArgs := re.condBuilder.Build(re.condition).
		WithPrefix(" AND ").
		WithStartArg(len(args) + 1).
		ToSQL()

	sql := `
		SELECT
			setting_name,
            setting_type,
            setting_value
		FROM
			` + re.table.Name + `
		WHERE
			` + re.table.PrimaryKey + ` = $1` + whereStr + `
		LIMIT 1;`

	row := entity.Setting{
		ID: id,
	}

	err := re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		mrsql.MergeArgs(args, whereArgs)...,
	).Scan(
		&row.Name,
		&row.Type,
		&row.Value,
	)
	if err != nil {
		return entity.Setting{}, re.errorWrapper.WrapErrorEntity(err, re.table.Name, mrmsg.Data{re.table.PrimaryKey: id})
	}

	return row, nil
}

// Update - обновляет указанную настройку.
func (re *SettingPostgres) Update(ctx context.Context, row entity.Setting) error {
	args := []any{
		row.Name,
		row.Type,
		row.Value,
	}

	whereStr, whereArgs := re.condBuilder.Build(re.condition).
		WithPrefix(" AND ").
		WithStartArg(len(args) + 1).
		ToSQL()

	sql := `
		UPDATE
			` + re.table.Name + `
		SET
			setting_value = $3,
			updated_at = NOW()
		WHERE
			` + re.table.PrimaryKey + ` = $1 AND setting_type = $2` + whereStr + `;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		mrsql.MergeArgs(args, whereArgs)...,
	)
	if err != nil {
		// TODO: добавить ошибку, если не удалось обновить настройку из-за условия WHERE
		return re.errorWrapper.WrapErrorEntity(err, re.table.Name, mrmsg.Data{re.table.PrimaryKey: row.Name})
	}

	return err
}
