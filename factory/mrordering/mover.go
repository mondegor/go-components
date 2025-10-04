package mrordering

import (
	"github.com/mondegor/go-storage/mrpostgres/builder/part"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrevent"

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
	useCaseErrorWrapper mrerr.UseCaseErrorWrapper,
	storageErrorWrapper mrerr.ErrorWrapper,
	eventEmitter mrevent.Emitter,
	storageTable mrsql.DBTableInfo,
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
			storageErrorWrapper,
			storageTable,
			part.NewSQLConditionBuilder(),
			o.storageCondition,
		),
		eventEmitter,
		useCaseErrorWrapper,
	)
}
