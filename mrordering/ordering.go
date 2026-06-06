package mrordering

import (
	"context"

	"github.com/mondegor/go-sysmess/mrstorage"
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
)
