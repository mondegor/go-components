package mrcom_orderer

import (
    "context"

    "github.com/mondegor/go-storage/mrentity"
)

type (
    EntityMeta interface {
        TableName() string
        PrimaryName() string
        ForEachCond(func (cond any))
    }

    Component interface {
        WithMetaData(meta EntityMeta) Component
        InsertToFirst(ctx context.Context, nodeId mrentity.KeyInt32) error
        InsertToLast(ctx context.Context, nodeId mrentity.KeyInt32) error
        MoveToLast(ctx context.Context, nodeId mrentity.KeyInt32) error
        MoveToFirst(ctx context.Context, nodeId mrentity.KeyInt32) error
        MoveAfterId(ctx context.Context, nodeId mrentity.KeyInt32, afterNodeId mrentity.KeyInt32) error
        Unlink(ctx context.Context, nodeId mrentity.KeyInt32) error
    }

    Storage interface {
        WithMetaData(meta EntityMeta) Storage
        LoadNode(ctx context.Context, row *EntityNode) error
        LoadFirstNode(ctx context.Context, row *EntityNode) error
        LoadLastNode(ctx context.Context, row *EntityNode) error
        UpdateNode(ctx context.Context, row *EntityNode) error
        UpdateNodePrevId(ctx context.Context, id mrentity.KeyInt32, prevId mrentity.ZeronullInt32) error
        UpdateNodeNextId(ctx context.Context, id mrentity.KeyInt32, nextId mrentity.ZeronullInt32) error
        RecalcOrderField(ctx context.Context, minBorder mrentity.Int64, step mrentity.Int64) error
    }
)
