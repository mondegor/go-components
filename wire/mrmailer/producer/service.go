package producer

import (
	"github.com/mondegor/go-sysmess/mrpostgres/sequence"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-sysmess/mrstorage/mrsql"
	"github.com/mondegor/go-sysmess/mrtrace"

	"github.com/mondegor/go-components/mrmailer/repository"
	"github.com/mondegor/go-components/mrmailer/service/produce"
	queuerepository "github.com/mondegor/go-components/mrqueue/repository"
	queueproduce "github.com/mondegor/go-components/mrqueue/service/produce"
)

// InitService - создаёт отправителя персонализированных уведомлений получателям.
func InitService(
	client mrstorage.DBConnManager,
	traceManager mrtrace.ContextManager,
	messageTable mrsql.DBTableInfo,
	queueTable mrsql.DBTableInfo,
	opts ...produce.Option,
) *produce.MessageProducer {
	return produce.New(
		client,
		sequence.NewGenerator(client, mrsql.SequenceName(queueTable)),
		repository.NewMessagePostgres(client, messageTable),
		queueproduce.New(
			queuerepository.NewQueuePostgres(client, queueTable),
		),
		traceManager,
		opts...,
	)
}
