package mrcom_orderer

import (
    "context"

    "github.com/Masterminds/squirrel"
    "github.com/mondegor/go-storage/mrentity"
    "github.com/mondegor/go-storage/mrstorage"
    "github.com/mondegor/go-sysmess/mrerr"
    "github.com/mondegor/go-webcore/mrcore"
)

// go get -u github.com/Masterminds/squirrel

type (
    repository struct {
        client mrstorage.DbConn
        builder squirrel.StatementBuilderType
        meta EntityMeta
    }
)

// NewRepository -
func NewRepository(client mrstorage.DbConn,
                   queryBuilder squirrel.StatementBuilderType) *repository {
    return &repository{
        client: client,
        builder: queryBuilder,
    }
}

// WithMetaData -
func (re *repository) WithMetaData(meta EntityMeta) Storage {
    return &repository{
        client:  re.client,
        builder: re.builder,
        meta:    meta,
    }
}

// LoadNode -
func (re *repository) LoadNode(ctx context.Context, row *EntityNode) error {
    if re.meta == nil {
        return mrcore.FactoryErrInternalNilPointer.New()
    }

    query := re.builder.
        Select(`
            prev_field_id,
            next_field_id,
            order_field`).
        From(re.meta.TableName()).
        Where(squirrel.Eq{re.meta.PrimaryName(): row.Id})

    re.meta.ForEachCond(func (cond any) { query = query.Where(cond) } )

    return re.client.SqQueryRow(ctx, query).Scan(&row.PrevId, &row.NextId, &row.OrderField)
}

// LoadFirstNode -
func (re *repository) LoadFirstNode(ctx context.Context, row *EntityNode) error {
    if re.meta == nil {
        return mrcore.FactoryErrInternalNilPointer.New()
    }

    query := re.builder.
        Select(`MIN(order_field)`).
        From(re.meta.TableName())

    re.meta.ForEachCond(func (cond any) { query = query.Where(cond) } )

    err := re.client.SqQueryRow(ctx, query).Scan(&row.OrderField)

    if err != nil {
        return err
    }

    err = re.loadNodeByOrderField(ctx, row)

    if err != nil {
        return err
    }

    if row.PrevId > 0 {
        return mrcore.FactoryErrStorageFetchedInvalidData.New(mrerr.Arg{"row.Id": row.Id, "row.PrevId": row.PrevId})
    }

    return nil
}

// LoadLastNode -
func (re *repository) LoadLastNode(ctx context.Context, row *EntityNode) error {
    if re.meta == nil {
        return mrcore.FactoryErrInternalNilPointer.New()
    }

    query := re.builder.
        Select(`MAX(order_field)`).
        From(re.meta.TableName())

    re.meta.ForEachCond(func (cond any) { query = query.Where(cond) } )

    err := re.client.SqQueryRow(ctx, query).Scan(&row.OrderField)

    if err != nil {
        return err
    }

    err = re.loadNodeByOrderField(ctx, row)

    if err != nil {
        return err
    }

    if row.NextId > 0 {
        return mrcore.FactoryErrStorageFetchedInvalidData.New(mrerr.Arg{"row.Id": row.Id, "row.NextId": row.NextId})
    }

    return nil
}

// UpdateNode -
func (re *repository) UpdateNode(ctx context.Context, row *EntityNode) error {
    if re.meta == nil {
        return mrcore.FactoryErrInternalNilPointer.New()
    }

    query := re.builder.
        Update(re.meta.TableName()).
        SetMap(map[string]any{
            "prev_field_id": row.PrevId,
            "next_field_id": row.NextId,
            "order_field": row.OrderField,
        }).
        Where(squirrel.Eq{re.meta.PrimaryName(): row.Id})

    re.meta.ForEachCond(func (cond any) { query = query.Where(cond) } )

    err := re.client.SqExec(ctx, query)

    if err != nil {
        return mrcore.FactoryErrInternalNoticeDataContainer.Wrap(err, mrerr.Arg{re.meta.PrimaryName(): row.Id})
    }

    return err
}

// UpdateNodePrevId -
func (re *repository) UpdateNodePrevId(ctx context.Context, id mrentity.KeyInt32, prevId mrentity.ZeronullInt32) error {
    if re.meta == nil {
        return mrcore.FactoryErrInternalNilPointer.New()
    }

    query := re.builder.
        Update(re.meta.TableName()).
        Set("prev_field_id", prevId).
        Where(squirrel.Eq{re.meta.PrimaryName(): id})

    re.meta.ForEachCond(func (cond any) { query = query.Where(cond) } )

    err := re.client.SqExec(ctx, query)

    if err != nil {
        return mrcore.FactoryErrInternalNoticeDataContainer.Wrap(err, mrerr.Arg{re.meta.PrimaryName(): id})
    }

    return nil
}

// UpdateNodeNextId -
func (re *repository) UpdateNodeNextId(ctx context.Context, id mrentity.KeyInt32, nextId mrentity.ZeronullInt32) error {
    if re.meta == nil {
        return mrcore.FactoryErrInternalNilPointer.New()
    }

    query := re.builder.
        Update(re.meta.TableName()).
        Set("next_field_id", nextId).
        Where(squirrel.Eq{re.meta.PrimaryName(): id})

    re.meta.ForEachCond(func (cond any) { query = query.Where(cond) } )

    err := re.client.SqExec(ctx, query)

    if err != nil {
        return mrcore.FactoryErrInternalNoticeDataContainer.Wrap(err, mrerr.Arg{re.meta.PrimaryName(): id})
    }

    return nil
}

// RecalcOrderField -
func (re *repository) RecalcOrderField(ctx context.Context, minBorder mrentity.Int64, step mrentity.Int64) error {
    if re.meta == nil {
        return mrcore.FactoryErrInternalNilPointer.New()
    }

    query := re.builder.
        Update(re.meta.TableName()).
        Set("order_field", squirrel.Expr("order_field + ?", step)).
        Where(squirrel.Gt{"order_field": minBorder})

    re.meta.ForEachCond(func (cond any) { query = query.Where(cond) } )

    return re.client.SqExec(ctx, query)
}

func (re *repository) loadNodeByOrderField(ctx context.Context, row *EntityNode) error {
    query := re.builder.
        Select(re.meta.PrimaryName(), `
            prev_field_id,
            next_field_id`).
        From(re.meta.TableName()).
        Where(squirrel.Eq{"order_field": row.OrderField}).
        Suffix("FETCH FIRST 1 ROWS ONLY")

    re.meta.ForEachCond(func (cond any) { query = query.Where(cond) } )

    return re.client.SqQueryRow(ctx, query).Scan(&row.Id, &row.PrevId, &row.NextId)
}
