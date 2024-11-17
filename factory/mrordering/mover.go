package mrordering

import (
	"github.com/mondegor/go-storage/mrpostgres/builder/part"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-webcore/mrcore/mrapp"
	"github.com/mondegor/go-webcore/mrsender"

	"github.com/mondegor/go-components/mrordering/component/move"
	"github.com/mondegor/go-components/mrordering/repository"
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
	options := moverOptions{
		storageCondition: nil,
	}

	for _, opt := range opts {
		opt(&options)
	}

	return move.New(
		repository.NewRepository(
			client,
			storageTable,
			part.NewSQLConditionBuilder(),
			mrapp.NewStorageErrorWrapper(),
			options.storageCondition,
		),
		eventEmitter,
		mrapp.NewUseCaseErrorWrapper(),
	)
}
