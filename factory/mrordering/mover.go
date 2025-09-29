package mrordering

import (
	"github.com/mondegor/go-storage/mrpostgres/builder/part"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-webcore/mrsender"

	"github.com/mondegor/go-components/mrordering/repository"
	"github.com/mondegor/go-components/mrordering/usecase/move"
)

type (
	moverOptions struct {
		storageCondition mrstorage.SQLPartFunc
	}
)

// NewComponentMover - создаёт объект move.NodeMover.
func NewComponentMover(
	client mrstorage.DBConnManager,
	storageTable mrsql.DBTableInfo,
	eventEmitter mrsender.EventEmitter,
	opts ...MoverOption,
) *move.NodeMover {
	o := moverOptions{
		storageCondition: nil,
	}

	for _, opt := range opts {
		opt(&o)
	}

	return move.New(
		repository.NewRepository(
			client,
			storageTable,
			part.NewSQLConditionBuilder(),
			o.storageCondition,
		),
		eventEmitter,
	)
}
