package producer

import (
	"github.com/mondegor/go-storage/mrpostgres/sequence"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrevent"
	"github.com/mondegor/go-sysmess/mrtrace"

	"github.com/mondegor/go-components/mrmailer/entity"
	"github.com/mondegor/go-components/mrmailer/repository"
	"github.com/mondegor/go-components/mrmailer/usecase/produce"
	queuerepository "github.com/mondegor/go-components/mrqueue/repository"
	queueproduce "github.com/mondegor/go-components/mrqueue/usecase/produce"
)

// NewSender - создаёт отправителя персонализированных уведомлений получателям.
func NewSender(
	client mrstorage.DBConnManager,
	eventEmitter mrevent.Emitter,
	useCaseErrorWrapper mrerr.UseCaseErrorWrapper,
	traceManager mrtrace.ContextManager,
	messageTable mrsql.DBTableInfo,
	queueTable mrsql.DBTableInfo,
	opts ...produce.Option,
) *produce.MessageSender {
	return produce.New(
		client,
		sequence.NewGenerator(client, mrsql.SequenceName(queueTable)),
		repository.NewMessagePostgres(client, messageTable),
		queueproduce.New(
			queuerepository.NewQueuePostgres(client, queueTable),
			mrevent.NewSourceEmitter(eventEmitter, entity.ModelNameMessage),
			useCaseErrorWrapper,
		),
		useCaseErrorWrapper,
		traceManager,
		opts...,
	)
}
