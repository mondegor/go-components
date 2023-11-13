package mrorderer

import (
    "github.com/mondegor/go-storage/mrstorage"
)

type (
    entityMeta struct {
        tableName string
        primaryName string
        where mrstorage.SqlBuilderPart
    }
)

func NewEntityMeta(tableName, primaryName string, where mrstorage.SqlBuilderPart) *entityMeta {
    return &entityMeta{
        tableName: tableName,
        primaryName: primaryName,
        where: where,
    }
}

func (e *entityMeta) TableName() string {
    return e.tableName
}

func (e *entityMeta) PrimaryName() string {
    return e.primaryName
}

func (e *entityMeta) Where() mrstorage.SqlBuilderPart {
    return e.where
}
