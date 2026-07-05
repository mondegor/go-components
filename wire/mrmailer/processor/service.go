package processor

import (
	"time"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrlog"
	"github.com/mondegor/go-core/mrprocess/consume"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/mrstorage/mrsql"
	"github.com/mondegor/go-core/mrtrace"

	"github.com/mondegor/go-components/mrmailer/entity"
	"github.com/mondegor/go-components/mrmailer/infra/handler"
	"github.com/mondegor/go-components/mrmailer/repository"
	"github.com/mondegor/go-components/mrmailer/sendmessage/provider"
	queuerepository "github.com/mondegor/go-components/mrqueue/repository"
	queueconsume "github.com/mondegor/go-components/mrqueue/service/consume"
)

const (
	defaultCaptionPrefix        = "Mailer"
	defaultReadyTimeout         = 60 * time.Second
	defaultReadPeriod           = 30 * time.Second
	defaultConsumerReadTimeout  = 2 * time.Second
	defaultConsumerWriteTimeout = 3 * time.Second
	defaultHandlerTimeout       = 30 * time.Second
	defaultQueueSize            = 25
	defaultWorkersCount         = 1
)

// InitService - создаёт сервис для обработки и отправки сообщений и связанных с ним задачи.
func InitService(
	client mrstorage.DBConnManager,
	errorHandler errors.Handler,
	logger mrlog.Logger,
	traceManager mrtrace.ContextManager,
	messageTable mrsql.DBTableInfo,
	queueTable mrsql.DBTableInfo,
	opts ...Option,
) *consume.MessageProcessor[entity.Message] {
	o := options{
		processorOpts: []consume.Option[entity.Message]{
			consume.WithCaptionPrefix[entity.Message](defaultCaptionPrefix),
			consume.WithReadyTimeout[entity.Message](defaultReadyTimeout),
			consume.WithReadPeriod[entity.Message](defaultReadPeriod),
			consume.WithConsumerTimeout[entity.Message](defaultConsumerReadTimeout, defaultConsumerWriteTimeout),
			consume.WithHandlerTimeout[entity.Message](defaultHandlerTimeout),
			consume.WithQueueSize[entity.Message](defaultQueueSize),
			consume.WithWorkersCount[entity.Message](defaultWorkersCount),
		},
		providerOpts: nil,
	}

	for _, opt := range opts {
		opt(&o)
	}

	storageMessage := repository.NewMessagePostgres(client, messageTable)

	storageQueue := queuerepository.NewQueuePostgres(client, queueTable)
	storageQueueCompleted := queuerepository.NewCompletedPostgres(
		client,
		mrsql.DBTableInfo{
			Name:       queueTable.Name + "_completed",
			PrimaryKey: queueTable.PrimaryKey,
		},
	)
	storageQueueCrashed := queuerepository.NewCrashedPostgres(
		client,
		mrsql.DBTableInfo{
			Name:       queueTable.Name + "_errors",
			PrimaryKey: queueTable.PrimaryKey,
		},
	)

	messageConsumer := queueconsume.NewMessageConsumer[entity.Message](
		client,
		storageMessage,
		queueconsume.NewQueueConsumer(
			client,
			storageQueue,
			queueconsume.WithStorageCompleted(storageQueueCompleted),
			queueconsume.WithStorageCrashed(storageQueueCrashed),
		),
	)

	return consume.NewMessageProcessor[entity.Message](
		messageConsumer,
		handler.NewSendMessage(
			provider.New(o.providerOpts...),
		),
		errorHandler,
		logger,
		traceManager,
		o.processorOpts...,
	)
}
