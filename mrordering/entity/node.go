package entity

import (
	"github.com/mondegor/go-core/mrentity"
)

const (
	// ModelNameNode - название сущности.
	ModelNameNode = "mrordering.Node"
)

type (
	// Node - элемент двусвязного списка участвующий в сортировке.
	Node struct {
		ID         uint64
		PrevID     mrentity.ZeronullUint64
		NextID     mrentity.ZeronullUint64
		OrderIndex mrentity.ZeronullUint64
	}
)
