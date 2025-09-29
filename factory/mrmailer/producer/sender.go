package producer

import (
	"github.com/mondegor/go-storage/mrpostgres/sequence"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-webcore/mrsender"
	"github.com/mondegor/go-webcore/mrsender/decorator"

	"github.com/mondegor/go-components/mrmailer/entity"
	"github.com/mondegor/go-components/mrmailer/repository"
	"github.com/mondegor/go-components/mrmailer/usecase/produce"
	queuerepository "github.com/mondegor/go-components/mrqueue/repository"
	queueproduce "github.com/mondegor/go-components/mrqueue/usecase/produce"
)

// NewSender - создаёт отправителя персонализированных уведомлений получателям.
func NewSender(
	client mrstorage.DBConnManager,
	messageTable mrsql.DBTableInfo,
	queueTable mrsql.DBTableInfo,
	eventEmitter mrsender.EventEmitter,
	opts ...produce.Option,
) *produce.MessageSender {
	return produce.New(
		client,
		sequence.NewGenerator(client, mrsql.SequenceName(queueTable)),
		repository.NewMessagePostgres(client, messageTable),
		queueproduce.New(
			queuerepository.NewQueuePostgres(client, queueTable),
			decorator.NewSourceEmitter(eventEmitter, entity.ModelNameMessage),
		),
		opts...,
	)
}
