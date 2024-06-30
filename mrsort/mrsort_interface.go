package mrsort

import (
	"context"

	"github.com/mondegor/go-storage/mrentity"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-webcore/mrtype"

	"github.com/mondegor/go-components/mrsort/entity"
)

type (
	// Orderer - интерфейс управления порядком следования записей.
	Orderer interface {
		WithMetaData(meta mrstorage.MetaGetter) Orderer
		InsertToFirst(ctx context.Context, nodeID mrtype.KeyInt32) error
		InsertToLast(ctx context.Context, nodeID mrtype.KeyInt32) error
		MoveToLast(ctx context.Context, nodeID mrtype.KeyInt32) error
		MoveToFirst(ctx context.Context, nodeID mrtype.KeyInt32) error
		MoveAfterID(ctx context.Context, nodeID, afterNodeID mrtype.KeyInt32) error
		Unlink(ctx context.Context, nodeID mrtype.KeyInt32) error
	}

	// Storage - интерфейс для доступа к записям порядка следования и их модификации.
	Storage interface {
		WithMetaData(meta mrstorage.MetaGetter) Storage
		FetchNode(ctx context.Context, nodeID mrtype.KeyInt32) (entity.Node, error)
		FetchFirstNode(ctx context.Context) (entity.Node, error)
		FetchLastNode(ctx context.Context) (entity.Node, error)
		UpdateNode(ctx context.Context, row entity.Node) error
		UpdateNodePrevID(ctx context.Context, rowID mrtype.KeyInt32, prevID mrentity.ZeronullInt32) error
		UpdateNodeNextID(ctx context.Context, rowID mrtype.KeyInt32, nextID mrentity.ZeronullInt32) error
		RecalcOrderIndex(ctx context.Context, minBorder, step int64) error
	}
)
