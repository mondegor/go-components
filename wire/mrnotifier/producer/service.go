package producer

import (
	"github.com/mondegor/go-storage/mrpostgres/sequence"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrtrace"

	"github.com/mondegor/go-components/mrnotifier/notifier/repository"
	"github.com/mondegor/go-components/mrnotifier/notifier/service/produce"
	queuerepository "github.com/mondegor/go-components/mrqueue/repository"
	queueproduce "github.com/mondegor/go-components/mrqueue/service/produce"
)

// InitService - создаёт отправителя сообщений получателям.
func InitService(
	client mrstorage.DBConnManager,
	traceManager mrtrace.ContextManager,
	noticeTable mrsql.DBTableInfo,
	queueTable mrsql.DBTableInfo,
	opts ...produce.Option,
) *produce.NoteProducer {
	return produce.New(
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
