package producer

import (
	"github.com/mondegor/go-core/mrpostgres/sequence"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/mrstorage/mrsql"
	"github.com/mondegor/go-core/mrtrace"

	"github.com/mondegor/go-components/mrnotifier/notifier/repository"
	"github.com/mondegor/go-components/mrnotifier/notifier/service"
	queuerepository "github.com/mondegor/go-components/mrqueue/repository"
	queueproduce "github.com/mondegor/go-components/mrqueue/service/produce"
)

// InitService - создаёт отправителя персонализированных уведомлений получателям.
func InitService(
	client mrstorage.DBConnManager,
	traceManager mrtrace.ContextManager,
	noticeTable mrsql.DBTableInfo,
	queueTable mrsql.DBTableInfo,
	opts ...service.Option,
) *service.NoteProducer {
	return service.New(
		client,
		sequence.NewGenerator(client, mrsql.SequenceName(queueTable)),
		repository.NewNotePostgres(client, noticeTable),
		queueproduce.New(
			queuerepository.NewQueuePostgres(client, queueTable),
		),
		traceManager,
		opts...,
	)
}
