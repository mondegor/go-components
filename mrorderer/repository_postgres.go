package mrorderer

import (
	"context"

	"github.com/mondegor/go-storage/mrentity"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrmsg"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrtype"
)

type (
	repository struct {
		client mrstorage.DBConn
		meta   EntityMeta
	}
)

// NewRepository -
func NewRepository(
	client mrstorage.DBConn,
) *repository {
	return &repository{
		client: client,
	}
}

// WithMetaData -
func (re *repository) WithMetaData(meta EntityMeta) Storage {
	c := *re
	c.meta = meta

	return &c
}

// LoadNode -
func (re *repository) LoadNode(ctx context.Context, row *EntityNode) error {
	if re.meta == nil {
		return mrcore.FactoryErrInternalNilPointer.New()
	}

	args := []any{
		row.ID,
	}

	whereStr, whereArgs, err := re.where(" AND ", len(args)+1)

	if err != nil {
		return err
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

	err = re.client.QueryRow(
		ctx,
		sql,
		mrsql.MergeArgs(args, whereArgs)...,
	).Scan(
		&row.PrevID,
		&row.NextID,
		&row.OrderIndex,
	)

	if err != nil {
		return mrcore.FactoryErrWithData.Wrap(err, re.meta.TableName(), mrmsg.Data{re.meta.PrimaryName(): row.ID})
	}

	return nil
}

// LoadFirstNode -
func (re *repository) LoadFirstNode(ctx context.Context, row *EntityNode) error {
	whereStr, whereArgs, err := re.where(" WHERE ", 1)

	if err != nil {
		return err
	}

	sql := `
		SELECT
			MIN(order_index)
		FROM
			` + re.meta.TableName() + whereStr + `
		LIMIT 1;`

	err = re.client.QueryRow(
		ctx,
		sql,
		whereArgs...,
	).Scan(
		&row.OrderIndex,
	)

	if err != nil {
		return mrcore.FactoryErrWithData.Wrap(err, re.meta.TableName(), "MIN(order_index)")
	}

	if err = re.loadNodeByOrderIndex(ctx, row); err != nil {
		return err
	}

	if row.PrevID > 0 {
		return mrcore.FactoryErrInternal.WithAttr(re.meta.TableName(), mrmsg.Data{"row.Id": row.ID, "row.PrevId": row.PrevID}).New()
	}

	return nil
}

// LoadLastNode -
func (re *repository) LoadLastNode(ctx context.Context, row *EntityNode) error {
	whereStr, whereArgs, err := re.where(" WHERE ", 1)

	if err != nil {
		return err
	}

	sql := `
		SELECT
			MAX(order_index)
		FROM
			` + re.meta.TableName() + whereStr + `
		LIMIT 1;`

	err = re.client.QueryRow(
		ctx,
		sql,
		whereArgs...,
	).Scan(
		&row.OrderIndex,
	)

	if err != nil {
		return mrcore.FactoryErrWithData.Wrap(err, re.meta.TableName(), "MAX(order_index)")
	}

	if row.OrderIndex == 0 {
		return nil
	}

	if err = re.loadNodeByOrderIndex(ctx, row); err != nil {
		return err
	}

	if row.NextID > 0 {
		return mrcore.FactoryErrInternal.WithAttr(re.meta.TableName(), mrmsg.Data{"row.Id": row.ID, "row.NextId": row.NextID}).New()
	}

	return nil
}

// UpdateNode -
func (re *repository) UpdateNode(ctx context.Context, row *EntityNode) error {
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

	err = re.client.Exec(
		ctx,
		sql,
		mrsql.MergeArgs(args, whereArgs)...,
	)

	if err != nil {
		return mrcore.FactoryErrWithData.Wrap(err, re.meta.TableName(), mrmsg.Data{re.meta.PrimaryName(): row.ID})
	}

	return err
}

// UpdateNodePrevID -
func (re *repository) UpdateNodePrevID(ctx context.Context, rowID mrtype.KeyInt32, prevID mrentity.ZeronullInt32) error {
	args := []any{
		rowID,
		prevID,
	}

	whereStr, whereArgs, err := re.where(" AND ", len(args)+1)

	if err != nil {
		return err
	}

	sql := `
		UPDATE
			` + re.meta.TableName() + `
		SET
			prev_field_id = $2
		WHERE
			` + re.meta.PrimaryName() + ` = $1` + whereStr + `;`

	err = re.client.Exec(
		ctx,
		sql,
		mrsql.MergeArgs(args, whereArgs)...,
	)

	if err != nil {
		return mrcore.FactoryErrWithData.Wrap(err, re.meta.TableName(), mrmsg.Data{re.meta.PrimaryName(): rowID})
	}

	return nil
}

// UpdateNodeNextID -
func (re *repository) UpdateNodeNextID(ctx context.Context, rowID mrtype.KeyInt32, nextID mrentity.ZeronullInt32) error {
	args := []any{
		rowID,
		nextID,
	}

	whereStr, whereArgs, err := re.where(" AND ", len(args)+1)

	if err != nil {
		return err
	}

	sql := `
		UPDATE
			` + re.meta.TableName() + `
		SET
			next_field_id = $2
		WHERE
			` + re.meta.PrimaryName() + ` = $1` + whereStr + `;`

	err = re.client.Exec(
		ctx,
		sql,
		mrsql.MergeArgs(args, whereArgs)...,
	)

	if err != nil {
		return mrcore.FactoryErrWithData.Wrap(err, re.meta.TableName(), mrmsg.Data{re.meta.PrimaryName(): rowID})
	}

	return nil
}

// RecalcOrderIndex -
func (re *repository) RecalcOrderIndex(ctx context.Context, minBorder, step int64) error {
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

	err = re.client.Exec(
		ctx,
		sql,
		mrsql.MergeArgs(args, whereArgs)...,
	)

	if err != nil {
		return mrcore.FactoryErrWithData.Wrap(err, re.meta.TableName(), mrmsg.Data{"order_index": minBorder, "step": step})
	}

	return nil
}

func (re *repository) loadNodeByOrderIndex(ctx context.Context, row *EntityNode) error {
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

	err = re.client.QueryRow(
		ctx,
		sql,
		mrsql.MergeArgs(args, whereArgs)...,
	).Scan(
		&row.ID,
		&row.PrevID,
		&row.NextID,
	)

	if err != nil {
		return mrcore.FactoryErrWithData.Wrap(err, re.meta.TableName(), mrmsg.Data{"order_index": row.OrderIndex})
	}

	return nil
}

func (re *repository) where(prefix string, paramNumber int) (string, []any, error) {
	if re.meta == nil {
		return "", nil, mrcore.FactoryErrInternalNilPointer.New()
	}

	str, args := re.meta.
		Where().
		WithPrefix(prefix).
		Param(paramNumber).
		ToSql()

	return str, args, nil
}
