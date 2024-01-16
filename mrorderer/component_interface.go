package mrorderer

import (
	"context"

	"github.com/mondegor/go-storage/mrentity"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-webcore/mrtype"
)

type (
	EntityMeta interface {
		TableName() string
		PrimaryName() string
		Where() mrstorage.SqlBuilderPart
	}

	API interface {
		WithMetaData(meta EntityMeta) API
		InsertToFirst(ctx context.Context, nodeID mrtype.KeyInt32) error
		InsertToLast(ctx context.Context, nodeID mrtype.KeyInt32) error
		MoveToLast(ctx context.Context, nodeID mrtype.KeyInt32) error
		MoveToFirst(ctx context.Context, nodeID mrtype.KeyInt32) error
		MoveAfterID(ctx context.Context, nodeID mrtype.KeyInt32, afterNodeID mrtype.KeyInt32) error
		Unlink(ctx context.Context, nodeID mrtype.KeyInt32) error
	}

	Storage interface {
		WithMetaData(meta EntityMeta) Storage
		LoadNode(ctx context.Context, row *EntityNode) error
		LoadFirstNode(ctx context.Context, row *EntityNode) error
		LoadLastNode(ctx context.Context, row *EntityNode) error
		UpdateNode(ctx context.Context, row *EntityNode) error
		UpdateNodePrevID(ctx context.Context, id mrtype.KeyInt32, prevID mrentity.ZeronullInt32) error
		UpdateNodeNextID(ctx context.Context, id mrtype.KeyInt32, nextID mrentity.ZeronullInt32) error
		RecalcOrderField(ctx context.Context, minBorder, step int64) error
	}
)
