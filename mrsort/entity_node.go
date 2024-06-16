package mrsort

import (
	"github.com/mondegor/go-storage/mrentity"
	"github.com/mondegor/go-webcore/mrtype"
)

const (
	ModelNameEntityOrderer = "EntityOrderer" // ModelNameEntityOrderer - название сущности
)

type (
	// EntityNode - элемент двусвязного списка участвующий в сортировке.
	EntityNode struct {
		ID         mrtype.KeyInt32
		PrevID     mrentity.ZeronullInt32
		NextID     mrentity.ZeronullInt32
		OrderIndex mrentity.ZeronullInt64
	}
)
