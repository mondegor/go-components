package mrnotifier

import (
	"github.com/mondegor/go-storage/mrpostgres/sequence"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-webcore/mrcore/mrapp"
	"github.com/mondegor/go-webcore/mrsender"
	"github.com/mondegor/go-webcore/mrsender/decorator"

	"github.com/mondegor/go-components/mrnotifier/notifier/component/produce"
	"github.com/mondegor/go-components/mrnotifier/notifier/entity"
	"github.com/mondegor/go-components/mrnotifier/notifier/repository"
	queueproduce "github.com/mondegor/go-components/mrqueue/component/produce"
	queuerepository "github.com/mondegor/go-components/mrqueue/repository"
)

// NewComponentSender - создаёт отправителя сообщений получателям.
func NewComponentSender(
	client mrstorage.DBConnManager,
	noticeTable mrsql.DBTableInfo,
	queueTable mrsql.DBTableInfo,
	eventEmitter mrsender.EventEmitter,
	opts ...produce.Option,
) *produce.NoticeSender {
	useCaseErrorWrapper := mrapp.NewUseCaseErrorWrapper()

	return produce.New(
		client,
		sequence.NewGenerator(client, mrsql.SequenceName(queueTable)),
		repository.NewNoticePostgres(client, noticeTable),
		queueproduce.New(
			queuerepository.NewQueuePostgres(client, queueTable),
			decorator.NewSourceEmitter(eventEmitter, entity.ModelNameNotice),
			useCaseErrorWrapper,
		),
		useCaseErrorWrapper,
		opts...,
	)
}
