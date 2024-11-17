package entity

import (
	"github.com/mondegor/go-storage/mrentity"
)

const (
	ModelNameNode = "mrordering.Node" // ModelNameNode - название сущности
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
