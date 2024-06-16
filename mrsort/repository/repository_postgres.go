package repository

import (
	"context"

	"github.com/mondegor/go-storage/mrentity"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrmsg"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrtype"

	"github.com/mondegor/go-components/mrsort"
)

type (
	// Repository - репозиторий для хранения порядка следования элементов.
	// В поле meta содержится информация о таблице, в которой должна быть реализована сортировка.
	Repository struct {
		client mrstorage.DBConnManager
		meta   mrstorage.MetaGetter
	}
)

// NewRepository - создаёт объект Repository.
func NewRepository(client mrstorage.DBConnManager) *Repository {
	return &Repository{
		client: client,
	}
}

// WithMetaData - comment method.
func (re *Repository) WithMetaData(meta mrstorage.MetaGetter) mrsort.Storage { //nolint:ireturn
	c := *re
	c.meta = meta

	return &c
}

// FetchNode - comment method.
func (re *Repository) FetchNode(ctx context.Context, nodeID mrtype.KeyInt32) (mrsort.EntityNode, error) {
	args := []any{
		nodeID,
	}

	whereStr, whereArgs, err := re.where(" AND ", len(args)+1)
	if err != nil {
		return mrsort.EntityNode{}, err
	}

	sql := `
		SELECT
			prev_field_id,
			next_field_id,
			order_index
		FROM
			` + re.meta.TableName() + `
		WHERE
			` + re.meta.PrimaryName() + ` = $1` + whereStr + `
		LIMIT 1;`

	row := mrsort.EntityNode{
		ID: nodeID,
	}

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		mrsql.MergeArgs(args, whereArgs)...,
	).Scan(
		&row.PrevID,
		&row.NextID,
		&row.OrderIndex,
	)
	if err != nil {
		return mrsort.EntityNode{}, re.wrapError(err, re.meta.TableName(), mrmsg.Data{re.meta.PrimaryName(): row.ID})
	}

	return row, nil
}

// FetchFirstNode - comment method.
func (re *Repository) FetchFirstNode(ctx context.Context) (mrsort.EntityNode, error) {
	whereStr, whereArgs, err := re.where(" WHERE ", 1)
	if err != nil {
		return mrsort.EntityNode{}, err
	}

	sql := `
		SELECT
			MIN(order_index)
		FROM
			` + re.meta.TableName() + whereStr + `
		LIMIT 1;`

	row := mrsort.EntityNode{}

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		whereArgs...,
	).Scan(
		&row.OrderIndex,
	)
	if err != nil {
		return mrsort.EntityNode{}, re.wrapError(err, re.meta.TableName(), "MIN(order_index)")
	}

	if err = re.loadNodeByOrderIndex(ctx, &row); err != nil {
		return mrsort.EntityNode{}, err
	}

	if row.PrevID > 0 {
		return mrsort.EntityNode{}, mrcore.ErrInternal.New().WithAttr(re.meta.TableName(), mrmsg.Data{"row.Id": row.ID, "row.PrevId": row.PrevID})
	}

	return row, nil
}

// FetchLastNode - comment method.
func (re *Repository) FetchLastNode(ctx context.Context) (mrsort.EntityNode, error) {
	whereStr, whereArgs, err := re.where(" WHERE ", 1)
	if err != nil {
		return mrsort.EntityNode{}, err
	}

	sql := `
		SELECT
			MAX(order_index)
		FROM
			` + re.meta.TableName() + whereStr + `
		LIMIT 1;`

	row := mrsort.EntityNode{}

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		whereArgs...,
	).Scan(
		&row.OrderIndex,
	)
	if err != nil {
		return mrsort.EntityNode{}, re.wrapError(err, re.meta.TableName(), "MAX(order_index)")
	}

	if row.OrderIndex == 0 {
		return mrsort.EntityNode{}, nil
	}

	if err = re.loadNodeByOrderIndex(ctx, &row); err != nil {
		return mrsort.EntityNode{}, err
	}

	if row.NextID > 0 {
		return mrsort.EntityNode{}, mrcore.ErrInternal.New().WithAttr(re.meta.TableName(), mrmsg.Data{"row.Id": row.ID, "row.NextId": row.NextID})
	}

	return row, nil
}

// UpdateNode - comment method.
func (re *Repository) UpdateNode(ctx context.Context, row mrsort.EntityNode) error {
	args := []any{
		row.ID,
		row.PrevID,
		row.NextID,
		row.OrderIndex,
	}

	whereStr, whereArgs, err := re.where(" AND ", len(args)+1)
	if err != nil {
		return err
	}

	sql := `
		UPDATE
			` + re.meta.TableName() + `
		SET
			prev_field_id = $2,
			next_field_id = $3,
			order_index = $4
		WHERE
			` + re.meta.PrimaryName() + ` = $1` + whereStr + `;`

	err = re.client.Conn(ctx).Exec(
		ctx,
		sql,
		mrsql.MergeArgs(args, whereArgs)...,
	)
	if err != nil {
		return re.wrapError(err, re.meta.TableName(), mrmsg.Data{re.meta.PrimaryName(): row.ID})
	}

	return err
}

// UpdateNodePrevID - comment method.
func (re *Repository) UpdateNodePrevID(ctx context.Context, rowID mrtype.KeyInt32, prevID mrentity.ZeronullInt32) error {
	return re.updateNodeNeighborID(ctx, rowID, prevID, "prev_")
}

// UpdateNodeNextID - comment method.
func (re *Repository) UpdateNodeNextID(ctx context.Context, rowID mrtype.KeyInt32, nextID mrentity.ZeronullInt32) error {
	return re.updateNodeNeighborID(ctx, rowID, nextID, "next_")
}

// RecalcOrderIndex - comment method.
func (re *Repository) RecalcOrderIndex(ctx context.Context, minBorder, step int64) error {
	args := []any{
		minBorder,
		step,
	}

	whereStr, whereArgs, err := re.where(" AND ", len(args)+1)
	if err != nil {
		return err
	}

	sql := `
		UPDATE
			` + re.meta.TableName() + `
		SET
			order_index = order_index + $2
		WHERE
			order_index > $1` + whereStr + `;`

	err = re.client.Conn(ctx).Exec(
		ctx,
		sql,
		mrsql.MergeArgs(args, whereArgs)...,
	)
	if err != nil {
		return re.wrapError(err, re.meta.TableName(), mrmsg.Data{"order_index": minBorder, "step": step})
	}

	return nil
}

func (re *Repository) loadNodeByOrderIndex(ctx context.Context, row *mrsort.EntityNode) error {
	args := []any{
		row.OrderIndex,
	}

	whereStr, whereArgs, err := re.where(" AND ", len(args)+1)
	if err != nil {
		return err
	}

	sql := `
		SELECT
			` + re.meta.PrimaryName() + `,
			prev_field_id,
			next_field_id
		FROM
			` + re.meta.TableName() + `
		WHERE
			order_index = $1` + whereStr + `
		ORDER BY
			` + re.meta.PrimaryName() + ` ASC
		LIMIT 1;`

	err = re.client.Conn(ctx).QueryRow(
		ctx,
		sql,
		mrsql.MergeArgs(args, whereArgs)...,
	).Scan(
		&row.ID,
		&row.PrevID,
		&row.NextID,
	)
	if err != nil {
		return re.wrapError(err, re.meta.TableName(), mrmsg.Data{"order_index": row.OrderIndex})
	}

	return nil
}

func (re *Repository) updateNodeNeighborID(ctx context.Context, rowID mrtype.KeyInt32, neighborID mrentity.ZeronullInt32, fieldPrefix string) error {
	args := []any{
		rowID,
		neighborID,
	}

	whereStr, whereArgs, err := re.where(" AND ", len(args)+1)
	if err != nil {
		return err
	}

	sql := `
		UPDATE
			` + re.meta.TableName() + `
		SET
			` + fieldPrefix + `field_id = $2
		WHERE
			` + re.meta.PrimaryName() + ` = $1` + whereStr + `;`

	err = re.client.Conn(ctx).Exec(
		ctx,
		sql,
		mrsql.MergeArgs(args, whereArgs)...,
	)
	if err != nil {
		return re.wrapError(err, re.meta.TableName(), mrmsg.Data{re.meta.PrimaryName(): rowID})
	}

	return nil
}

func (re *Repository) where(prefix string, paramNumber int) (string, []any, error) {
	if re.meta == nil {
		return "", nil, mrcore.ErrInternalNilPointer.New()
	}

	str, args := re.meta.
		Condition().
		WithPrefix(prefix).
		WithParam(paramNumber).
		ToSQL()

	return str, args, nil
}

func (re *Repository) wrapError(err error, tableName string, data any) error {
	return mrcore.CastToAppError(err).
		WithAttr("tableName", tableName).
		WithAttr("data", data)
}
