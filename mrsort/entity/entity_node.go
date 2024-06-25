package entity

import (
	"github.com/mondegor/go-storage/mrentity"
	"github.com/mondegor/go-webcore/mrtype"
)

const (
	ModelNameNode = "mrsort.Node" // ModelNameNode - название сущности
)

type (
	// Node - элемент двусвязного списка участвующий в сортировке.
	Node struct {
		ID         mrtype.KeyInt32
		PrevID     mrentity.ZeronullInt32
		NextID     mrentity.ZeronullInt32
		OrderIndex mrentity.ZeronullInt64
	}
)
