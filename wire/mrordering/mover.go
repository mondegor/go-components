package mrordering

import (
	"github.com/mondegor/go-sysmess/mrevent"
	"github.com/mondegor/go-sysmess/mrpostgres/builder/part"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-sysmess/mrstorage/mrsql"

	"github.com/mondegor/go-components/mrordering/repository"
	"github.com/mondegor/go-components/mrordering/service"
)

// InitServiceMover - создаёт объект move.NodeMover.
func InitServiceMover(
	client mrstorage.DBConnManager,
	eventEmitter mrevent.Emitter,
	storageTable mrsql.DBTableInfo,
	opts ...Option,
) *service.NodeMover {
	o := options{
		storageCondition: nil,
	}

	for _, opt := range opts {
		opt(&o)
	}

	return service.New(
		repository.NewNodePostgres(
			client,
			storageTable,
			part.NewSQLConditionBuilder(),
			o.storageCondition,
		),
		eventEmitter,
	)
}
