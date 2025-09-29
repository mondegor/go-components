package repository

import (
	"context"

	"github.com/mondegor/go-storage/mrentity"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrargs"
	"github.com/mondegor/go-sysmess/mrerr/mr"

	core "github.com/mondegor/go-components/internal"
	"github.com/mondegor/go-components/mrordering/entity"
)

type (
	// Repository - репозиторий для хранения порядка следования элементов.
	// В поле table содержится информация о таблице, в которой должны быть
	// выделены следующие поля предназначенные для сортировки:
	// - prev_field_id - предыдущий элемент, за которым следует текущий элемент;
	// - next_field_id - следующий элемент, перед которым расположен текущий элемент;
	// - order_index - поле порядка следования.
	Repository struct {
		client       mrstorage.DBConnManager
		table        mrsql.DBTableInfo
		whereBuilder mrstorage.SQLConditionBuilder
		condition    mrstorage.SQLPartFunc // OPTIONAL
		errorWrapper core.ErrorWrapper
	}
)

// NewRepository - создаёт объект Repository.
func NewRepository(
	client mrstorage.DBConnManager,
	table mrsql.DBTableInfo,
	whereBuilder mrstorage.SQLConditionBuilder,
	condition mrstorage.SQLPartFunc, // OPTIONAL
) *Repository {
	return &Repository{
		client:       client,
		table:        table,
		whereBuilder: whereBuilder,
		condition:    condition,
		errorWrapper: core.NewStorageErrorWrapper(table.Name),
	}
}

// FetchNode - возвращает элемент, по указанному SettingID с учётом указанного условия.
func (re *Repository) FetchNode(ctx context.Context, rowID uint64, condition mrstorage.SQLPartFunc) (entity.Node, error) {
	args := []any{
		rowID,
	}

	whereStr, whereArgs := re.whereBuilder.BuildAnd(re.condition, condition).
		WithPrefix(" AND ").
		WithStartArg(len(args) + 1).
		ToSQL()

	sql := `
		SELECT
			prev_field_id,
			next_field_id,
			order_index
		FROM
			` + re.table.Name + `
		WHERE
			` + re.table.PrimaryKey + ` = $1` + whereStr + `
		LIMIT 1;`

	row := entity.Node{
		ID: rowID,
	}

	err := re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		mrsql.MergeArgs(args, whereArgs)...,
	).Scan(
		&row.PrevID,
		&row.NextID,
		&row.OrderIndex,
	)
	if err != nil {
		return entity.Node{}, re.errorWrapper.WrapError(err, "storage-data", mrargs.Group{"id": row.ID})
	}

	return row, nil
}

// FetchFirstNode - возвращает первый элемент в списке с учётом указанного условия.
func (re *Repository) FetchFirstNode(ctx context.Context, condition mrstorage.SQLPartFunc) (entity.Node, error) {
	whereStr, whereArgs := re.whereBuilder.BuildAnd(re.condition, condition).
		WithPrefix(" WHERE ").
		ToSQL()

	sql := `
		SELECT
			MIN(order_index)
		FROM
			` + re.table.Name + whereStr + `
		LIMIT 1;`

	row := entity.Node{}

	err := re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		whereArgs...,
	).Scan(
		&row.OrderIndex,
	)
	if err != nil {
		return entity.Node{}, re.errorWrapper.WrapError(err, "storage-data", "MIN(order_index)")
	}

	if err = re.loadNodeByOrderIndex(ctx, &row, condition); err != nil {
		return entity.Node{}, err
	}

	if row.PrevID > 0 {
		return entity.Node{}, mr.ErrInternal.New().WithAttrs("source", re.table.Name, "data", mrargs.Group{"row.Id": row.ID, "row.PrevId": row.PrevID})
	}

	return row, nil
}

// FetchLastNode - возвращает последний элемент в списке с учётом указанного условия.
func (re *Repository) FetchLastNode(ctx context.Context, condition mrstorage.SQLPartFunc) (entity.Node, error) {
	whereStr, whereArgs := re.whereBuilder.BuildAnd(re.condition, condition).
		WithPrefix(" WHERE ").
		ToSQL()

	sql := `
		SELECT
			MAX(order_index)
		FROM
			` + re.table.Name + whereStr + `
		LIMIT 1;`

	row := entity.Node{}

	err := re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		whereArgs...,
	).Scan(
		&row.OrderIndex,
	)
	if err != nil {
		return entity.Node{}, re.errorWrapper.WrapError(err, "storage-data", "MAX(order_index)")
	}

	if row.OrderIndex == 0 {
		return entity.Node{}, nil
	}

	if err = re.loadNodeByOrderIndex(ctx, &row, condition); err != nil {
		return entity.Node{}, err
	}

	if row.NextID > 0 {
		return entity.Node{}, mr.ErrInternal.New().WithAttrs("source", re.table.Name, "data", mrargs.Group{"row.Id": row.ID, "row.NextId": row.NextID})
	}

	return row, nil
}

// UpdateNode - обновляет местоположение элемента в списке с учётом указанного условия.
func (re *Repository) UpdateNode(ctx context.Context, row entity.Node, condition mrstorage.SQLPartFunc) error {
	args := []any{
		row.ID,
		row.PrevID,
		row.NextID,
		row.OrderIndex,
	}

	whereStr, whereArgs := re.whereBuilder.BuildAnd(re.condition, condition).
		WithPrefix(" AND ").
		WithStartArg(len(args) + 1).
		ToSQL()

	sql := `
		UPDATE
			` + re.table.Name + `
		SET
			prev_field_id = $2,
			next_field_id = $3,
			order_index = $4
		WHERE
			` + re.table.PrimaryKey + ` = $1` + whereStr + `;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		mrsql.MergeArgs(args, whereArgs)...,
	)
	if err != nil {
		return re.errorWrapper.WrapError(err, "storage-data", mrargs.Group{"id": row.ID})
	}

	return err
}

// UpdateNodePrevID - обновляет местоположение элемента в списке с учётом указанного условия.
func (re *Repository) UpdateNodePrevID(ctx context.Context, rowID uint64, prevID mrentity.ZeronullUint64, condition mrstorage.SQLPartFunc) error {
	return re.updateNodeNeighborID(ctx, condition, rowID, prevID, "prev_")
}

// UpdateNodeNextID - comment method.
func (re *Repository) UpdateNodeNextID(ctx context.Context, rowID uint64, nextID mrentity.ZeronullUint64, condition mrstorage.SQLPartFunc) error {
	return re.updateNodeNeighborID(ctx, condition, rowID, nextID, "next_")
}

// RecalcOrderIndex - comment method.
func (re *Repository) RecalcOrderIndex(ctx context.Context, minBorder, step uint64, condition mrstorage.SQLPartFunc) error {
	args := []any{
		minBorder,
		step,
	}

	whereStr, whereArgs := re.whereBuilder.BuildAnd(re.condition, condition).
		WithPrefix(" AND ").
		WithStartArg(len(args) + 1).
		ToSQL()

	sql := `
		UPDATE
			` + re.table.Name + `
		SET
			order_index = order_index + $2
		WHERE
			order_index > $1` + whereStr + `;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		mrsql.MergeArgs(args, whereArgs)...,
	)
	if err != nil {
		return re.errorWrapper.WrapError(err, "storage-data", mrargs.Group{"orderIndex": minBorder, "step": step})
	}

	return nil
}

func (re *Repository) loadNodeByOrderIndex(ctx context.Context, row *entity.Node, condition mrstorage.SQLPartFunc) error {
	args := []any{
		row.OrderIndex,
	}

	whereStr, whereArgs := re.whereBuilder.BuildAnd(re.condition, condition).
		WithPrefix(" AND ").
		WithStartArg(len(args) + 1).
		ToSQL()

	sql := `
		SELECT
			` + re.table.PrimaryKey + `,
			prev_field_id,
			next_field_id
		FROM
			` + re.table.Name + `
		WHERE
			order_index = $1` + whereStr + `
		ORDER BY
			` + re.table.PrimaryKey + ` ASC
		LIMIT 1;`

	err := re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		mrsql.MergeArgs(args, whereArgs)...,
	).Scan(
		&row.ID,
		&row.PrevID,
		&row.NextID,
	)
	if err != nil {
		return re.errorWrapper.WrapError(err, "storage-data", mrargs.Group{"orderIndex": row.OrderIndex})
	}

	return nil
}

func (re *Repository) updateNodeNeighborID(
	ctx context.Context,
	condition mrstorage.SQLPartFunc,
	rowID uint64,
	neighborID mrentity.ZeronullUint64,
	fieldPrefix string,
) error {
	args := []any{
		rowID,
		neighborID,
	}

	whereStr, whereArgs := re.whereBuilder.BuildAnd(re.condition, condition).
		WithPrefix(" AND ").
		WithStartArg(len(args) + 1).
		ToSQL()

	sql := `
		UPDATE
			` + re.table.Name + `
		SET
			` + fieldPrefix + `field_id = $2
		WHERE
			` + re.table.PrimaryKey + ` = $1` + whereStr + `;`

	err := re.client.Conn(ctx).Exec(
		ctx,
		sql,
		mrsql.MergeArgs(args, whereArgs)...,
	)
	if err != nil {
		return re.errorWrapper.WrapError(err, "storage-data", mrargs.Group{"id": rowID})
	}

	return nil
}
