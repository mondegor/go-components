package mrmailer

import (
	"github.com/mondegor/go-storage/mrpostgres/sequence"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-webcore/mrcore/mrapp"
	"github.com/mondegor/go-webcore/mrsender"
	"github.com/mondegor/go-webcore/mrsender/decorator"

	"github.com/mondegor/go-components/mrmailer/component/produce"
	"github.com/mondegor/go-components/mrmailer/entity"
	"github.com/mondegor/go-components/mrmailer/repository"
	queueproduce "github.com/mondegor/go-components/mrqueue/component/produce"
	queuerepository "github.com/mondegor/go-components/mrqueue/repository"
)

// NewComponentSender - создаёт отправителя персонализированных уведомлений получателям.
func NewComponentSender(
	client mrstorage.DBConnManager,
	messageTable mrsql.DBTableInfo,
	queueTable mrsql.DBTableInfo,
	eventEmitter mrsender.EventEmitter,
	opts ...produce.Option,
) *produce.MessageSender {
	useCaseErrorWrapper := mrapp.NewUseCaseErrorWrapper()

	return produce.New(
		client,
		sequence.NewGenerator(client, mrsql.SequenceName(queueTable)),
		repository.NewMessagePostgres(client, messageTable),
		queueproduce.New(
			queuerepository.NewQueuePostgres(client, queueTable),
			decorator.NewSourceEmitter(eventEmitter, entity.ModelNameMessage),
			useCaseErrorWrapper,
		),
		useCaseErrorWrapper,
		opts...,
	)
}
