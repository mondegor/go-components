package mrcom_orderer

import "github.com/mondegor/go-storage/mrentity"

const ModelNameEntityOrderer = "EntityOrderer"

type (
    EntityNode struct {
        Id mrentity.KeyInt32
        PrevId mrentity.ZeronullInt32
        NextId mrentity.ZeronullInt32
        OrderField mrentity.ZeronullInt64
    }
)
