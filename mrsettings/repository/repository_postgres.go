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

type (
	// Repository - репозиторий для хранения элементов настроек.
	Repository struct {
		client    mrstorage.DBConnManager
		meta      mrstorage.MetaGetter
		condition mrstorage.SQLBuilderCondition
	}
)

// New - создаёт объект Repository.
func New(client mrstorage.DBConnManager, meta mrstorage.MetaGetter, condition mrstorage.SQLBuilderCondition) *Repository {
	return &Repository{
		client:    client,
		meta:      meta,
		condition: condition,
	}
}

// Fetch - comment method.
func (re *Repository) Fetch(ctx context.Context, lastUpdated time.Time) ([]entity.Setting, error) {
	whereLastUpdated := re.condition.Where(func(w mrstorage.SQLBuilderWhere) mrstorage.SQLBuilderPartFunc {
		return w.Greater("updated_at", lastUpdated)
	})

	whereStr, whereArgs, err := re.where(whereLastUpdated, 1)
	if err != nil {
		return nil, err
	}

	sql := `
		SELECT
			` + re.meta.PrimaryName() + `,
			setting_name,
			setting_type,
			setting_value,
			setting_description,
			updated_at
		FROM
			` + re.meta.TableName() + `
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

// FetchOne - comment method.
func (re *Repository) FetchOne(ctx context.Context, id uint64) (entity.Setting, error) {
	args := []any{
		id,
	}

	whereStr, whereArgs, err := re.whereMeta(" AND ", len(args)+1)
	if err != nil {
		return entity.Setting{}, err
	}

	sql := `
		SELECT
			setting_name,
            setting_type,
            setting_value
		FROM
			` + re.meta.TableName() + `
		WHERE
			` + re.meta.PrimaryName() + ` = $1` + whereStr + `
		LIMIT 1;`

	row := entity.Setting{
		ID: id,
	}

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		mrsql.MergeArgs(args, whereArgs)...,
	).Scan(
		&row.Name,
		&row.Type,
		&row.Value,
	)
	if err != nil {
		return entity.Setting{}, re.wrapError(err, re.meta.TableName(), mrmsg.Data{re.meta.PrimaryName(): id})
	}

	return row, nil
}

// Update - comment method.
func (re *Repository) Update(ctx context.Context, row entity.Setting) error {
	args := []any{
		row.Name,
		row.Type,
		row.Value,
	}

	whereStr, whereArgs, err := re.whereMeta(" AND ", len(args)+1)
	if err != nil {
		return err
	}

	sql := `
		UPDATE
			` + re.meta.TableName() + `
		SET
			setting_value = $3,
			updated_at = NOW()
		WHERE
			` + re.meta.PrimaryName() + ` = $1 AND setting_type = $2` + whereStr + `;`

	err = re.client.Conn(ctx).Exec(
		ctx,
		sql,
		mrsql.MergeArgs(args, whereArgs)...,
	)
	if err != nil {
		// :TODO: добавить ошибку, если не удалось обновить настройку из-за условия WHERE
		return re.wrapError(err, re.meta.TableName(), mrmsg.Data{re.meta.PrimaryName(): row.Name})
	}

	return err
}

func (re *Repository) whereMeta(prefix string, paramNumber int) (string, []any, error) {
	if re.meta == nil {
		return "", nil, mrcore.ErrInternalNilPointer.New()
	}

	str, args := re.meta.Condition().
		WithPrefix(prefix).
		WithParam(paramNumber).
		ToSQL()

	return str, args, nil
}

func (re *Repository) where(part mrstorage.SQLBuilderPart, paramNumber int) (string, []any, error) {
	if re.meta == nil {
		return "", nil, mrcore.ErrInternalNilPointer.New()
	}

	str, args := part.
		WithPart(" AND ", re.meta.Condition()).
		WithParam(paramNumber).
		ToSQL()

	return str, args, nil
}

func (re *Repository) wrapError(err error, tableName string, data any) error {
	return mrcore.CastToAppError(err).
		WithAttr("tableName", tableName).
		WithAttr("data", data)
}
