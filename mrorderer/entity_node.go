package mrorderer

import (
	"github.com/mondegor/go-storage/mrentity"
	"github.com/mondegor/go-webcore/mrtype"
)

const (
	ModelNameEntityOrderer = "EntityOrderer"
)

type (
	EntityNode struct {
		ID         mrtype.KeyInt32
		PrevID     mrentity.ZeronullInt32
		NextID     mrentity.ZeronullInt32
		OrderField mrentity.ZeronullInt64
	}
)
