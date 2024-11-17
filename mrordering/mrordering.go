package mrordering

import (
	"context"

	"github.com/mondegor/go-storage/mrentity"
	"github.com/mondegor/go-storage/mrstorage"

	"github.com/mondegor/go-components/mrordering/entity"
)

type (
	// Mover - управляет порядком следования элементов.
	Mover interface {
		InsertToFirst(ctx context.Context, nodeID uint64, condition mrstorage.SQLPartFunc) error
		InsertToLast(ctx context.Context, nodeID uint64, condition mrstorage.SQLPartFunc) error
		MoveToLast(ctx context.Context, nodeID uint64, condition mrstorage.SQLPartFunc) error
		MoveToFirst(ctx context.Context, nodeID uint64, condition mrstorage.SQLPartFunc) error
		MoveAfterID(ctx context.Context, nodeID, afterNodeID uint64, condition mrstorage.SQLPartFunc) error
		Unlink(ctx context.Context, nodeID uint64, condition mrstorage.SQLPartFunc) error
	}

	// Storage - доступ служебным данным используемым для построения порядка следования элементов.
	Storage interface {
		FetchNode(ctx context.Context, rowID uint64, condition mrstorage.SQLPartFunc) (entity.Node, error)
		FetchFirstNode(ctx context.Context, condition mrstorage.SQLPartFunc) (entity.Node, error)
		FetchLastNode(ctx context.Context, condition mrstorage.SQLPartFunc) (entity.Node, error)
		UpdateNode(ctx context.Context, row entity.Node, condition mrstorage.SQLPartFunc) error
		UpdateNodePrevID(ctx context.Context, rowID uint64, prevID mrentity.ZeronullUint64, condition mrstorage.SQLPartFunc) error
		UpdateNodeNextID(ctx context.Context, rowID uint64, nextID mrentity.ZeronullUint64, condition mrstorage.SQLPartFunc) error
		RecalcOrderIndex(ctx context.Context, minBorder, step uint64, condition mrstorage.SQLPartFunc) error
	}
)
