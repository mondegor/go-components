package mrsort

import (
	"context"

	"github.com/mondegor/go-storage/mrentity"
	"github.com/mondegor/go-storage/mrstorage"

	"github.com/mondegor/go-components/mrsort/entity"
)

type (
	// Orderer - интерфейс управления порядком следования записей.
	Orderer interface {
		WithMetaData(meta mrstorage.MetaGetter) Orderer
		InsertToFirst(ctx context.Context, nodeID uint64) error
		InsertToLast(ctx context.Context, nodeID uint64) error
		MoveToLast(ctx context.Context, nodeID uint64) error
		MoveToFirst(ctx context.Context, nodeID uint64) error
		MoveAfterID(ctx context.Context, nodeID, afterNodeID uint64) error
		Unlink(ctx context.Context, nodeID uint64) error
	}

	// Storage - интерфейс для доступа к записям порядка следования и их модификации.
	Storage interface {
		WithMetaData(meta mrstorage.MetaGetter) Storage
		FetchNode(ctx context.Context, nodeID uint64) (entity.Node, error)
		FetchFirstNode(ctx context.Context) (entity.Node, error)
		FetchLastNode(ctx context.Context) (entity.Node, error)
		UpdateNode(ctx context.Context, row entity.Node) error
		UpdateNodePrevID(ctx context.Context, rowID uint64, prevID mrentity.ZeronullUint64) error
		UpdateNodeNextID(ctx context.Context, rowID uint64, nextID mrentity.ZeronullUint64) error
		RecalcOrderIndex(ctx context.Context, minBorder, step uint64) error
	}
)
