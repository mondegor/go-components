package producer

import (
	"github.com/mondegor/go-storage/mrpostgres/sequence"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-webcore/mrsender"
	"github.com/mondegor/go-webcore/mrsender/decorator"

	"github.com/mondegor/go-components/mrnotifier/notifier/entity"
	"github.com/mondegor/go-components/mrnotifier/notifier/repository"
	"github.com/mondegor/go-components/mrnotifier/notifier/usecase/produce"
	queuerepository "github.com/mondegor/go-components/mrqueue/repository"
	queueproduce "github.com/mondegor/go-components/mrqueue/usecase/produce"
)

// NewSender - создаёт отправителя сообщений получателям.
func NewSender(
	client mrstorage.DBConnManager,
	noticeTable mrsql.DBTableInfo,
	queueTable mrsql.DBTableInfo,
	eventEmitter mrsender.EventEmitter,
	opts ...produce.Option,
) *produce.NoticeSender {
	return produce.New(
		client,
		sequence.NewGenerator(client, mrsql.SequenceName(queueTable)),
		repository.NewNoticePostgres(client, noticeTable),
		queueproduce.New(
			queuerepository.NewQueuePostgres(client, queueTable),
			decorator.NewSourceEmitter(eventEmitter, entity.ModelNameNotice),
		),
		opts...,
	)
}
