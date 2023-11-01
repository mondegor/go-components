package mrorderer

import (
    "context"

    "github.com/mondegor/go-storage/mrentity"
    "github.com/mondegor/go-storage/mrsql"
    "github.com/mondegor/go-storage/mrstorage"
    "github.com/mondegor/go-sysmess/mrerr"
    "github.com/mondegor/go-webcore/mrcore"
)

type (
    repository struct {
        client mrstorage.DbConn
        meta EntityMeta
    }
)

// NewRepository -
func NewRepository(
    client mrstorage.DbConn,
) *repository {
    return &repository{
        client: client,
    }
}

// WithMetaData -
func (re *repository) WithMetaData(meta EntityMeta) Storage {
    return &repository{
        client:  re.client,
        meta:    meta,
    }
}

// LoadNode -
func (re *repository) LoadNode(ctx context.Context, row *EntityNode) error {
    if re.meta == nil {
        return mrcore.FactoryErrInternalNilPointer.New()
    }

    args := []any{
        row.Id,
    }

    whereStr, whereArgs, err := re.where(" AND ", len(args) + 1)

    if err != nil {
        return err
    }

    sql := `
        SELECT
            prev_field_id,
            next_field_id,
            order_field
        FROM
            ` + re.meta.TableName() + `
        WHERE ` + re.meta.PrimaryName() + ` = $1` + whereStr + `;`

    return re.client.QueryRow(
        ctx,
        sql,
        mrsql.MergeArgs(args, whereArgs)...,
    ).Scan(
        &row.PrevId,
        &row.NextId,
        &row.OrderField,
    )
}

// LoadFirstNode -
func (re *repository) LoadFirstNode(ctx context.Context, row *EntityNode) error {
    whereStr, whereArgs, err := re.where(" WHERE ", 1)

    if err != nil {
        return err
    }

    sql := `
        SELECT
            MIN(order_field)
        FROM
            ` + re.meta.TableName() + whereStr + `;`

    err = re.client.QueryRow(
        ctx,
        sql,
        whereArgs...,
    ).Scan(
        &row.OrderField,
    )

    if err != nil {
        return err
    }

    if err = re.loadNodeByOrderField(ctx, row); err != nil {
        return err
    }

    if row.PrevId > 0 {
        return mrcore.FactoryErrStorageFetchedInvalidData.New(mrerr.Arg{"row.Id": row.Id, "row.PrevId": row.PrevId})
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
            MAX(order_field)
        FROM
            ` + re.meta.TableName() + whereStr + `;`

    err = re.client.QueryRow(
        ctx,
        sql,
        whereArgs...,
    ).Scan(
        &row.OrderField,
    )

    if err != nil {
        return err
    }

    if err = re.loadNodeByOrderField(ctx, row); err != nil {
        return err
    }

    if row.NextId > 0 {
        return mrcore.FactoryErrStorageFetchedInvalidData.New(mrerr.Arg{"row.Id": row.Id, "row.NextId": row.NextId})
    }

    return nil
}

// UpdateNode -
func (re *repository) UpdateNode(ctx context.Context, row *EntityNode) error {
    args := []any{
        row.Id,
        row.PrevId,
        row.NextId,
        row.OrderField,
    }

    whereStr, whereArgs, err := re.where(" AND ", len(args) + 1)

    if err != nil {
        return err
    }

    sql := `
        UPDATE ` + re.meta.TableName() + `
        SET
            prev_field_id = $2,
            next_field_id = $3,
            order_field = $4
        WHERE
            ` + re.meta.PrimaryName() + ` = $1` + whereStr + `;`

    err = re.client.Exec(
        ctx,
        sql,
        mrsql.MergeArgs(args, whereArgs)...
    )

    if err != nil {
        return mrcore.FactoryErrInternalNoticeDataContainer.Wrap(err, mrerr.Arg{re.meta.PrimaryName(): row.Id})
    }

    return err
}

// UpdateNodePrevId -
func (re *repository) UpdateNodePrevId(ctx context.Context, id mrentity.KeyInt32, prevId mrentity.ZeronullInt32) error {
    args := []any{
        id,
        prevId,
    }

    whereStr, whereArgs, err := re.where(" AND ", len(args) + 1)

    if err != nil {
        return err
    }

    sql := `
        UPDATE ` + re.meta.TableName() + `
        SET
            prev_field_id = $2
        WHERE
            ` + re.meta.PrimaryName() + ` = $1` + whereStr + `;`

    err = re.client.Exec(
        ctx,
        sql,
        mrsql.MergeArgs(args, whereArgs)...
    )

    if err != nil {
        return mrcore.FactoryErrInternalNoticeDataContainer.Wrap(err, mrerr.Arg{re.meta.PrimaryName(): id})
    }

    return nil
}

// UpdateNodeNextId -
func (re *repository) UpdateNodeNextId(ctx context.Context, id mrentity.KeyInt32, nextId mrentity.ZeronullInt32) error {
    args := []any{
        id,
        nextId,
    }

    whereStr, whereArgs, err := re.where(" AND ", len(args) + 1)

    if err != nil {
        return err
    }

    sql := `
        UPDATE ` + re.meta.TableName() + `
        SET
            next_field_id = $2
        WHERE
            ` + re.meta.PrimaryName() + ` = $1` + whereStr + `;`

    err = re.client.Exec(
        ctx,
        sql,
        mrsql.MergeArgs(args, whereArgs)...
    )

    if err != nil {
        return mrcore.FactoryErrInternalNoticeDataContainer.Wrap(err, mrerr.Arg{re.meta.PrimaryName(): id})
    }

    return nil
}

// RecalcOrderField -
func (re *repository) RecalcOrderField(ctx context.Context, minBorder mrentity.Int64, step mrentity.Int64) error {
    args := []any{
        minBorder,
        step,
    }

    whereStr, whereArgs, err := re.where(" AND ", len(args) + 1)

    if err != nil {
        return err
    }

    sql := `
        UPDATE ` + re.meta.TableName() + `
        SET
            order_field = order_field + $2
        WHERE
            order_field > $1` + whereStr + `;`

    return re.client.Exec(
        ctx,
        sql,
        mrsql.MergeArgs(args, whereArgs)...
    )
}

func (re *repository) loadNodeByOrderField(ctx context.Context, row *EntityNode) error {
    whereStr, whereArgs, err := re.where(" AND ", 1)

    if err != nil {
        return err
    }

    sql := `
        SELECT
            ` + re.meta.PrimaryName() + `
            prev_field_id,
            next_field_id
        FROM
            ` + re.meta.TableName() + `
        WHERE order_field = $1` + whereStr + `
        FETCH FIRST 1 ROWS ONLY;`

    return re.client.QueryRow(
        ctx,
        sql,
        mrsql.MergeArgs([]any{row.OrderField}, whereArgs)...,
    ).Scan(
        &row.Id,
        &row.PrevId,
        &row.NextId,
    )
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
