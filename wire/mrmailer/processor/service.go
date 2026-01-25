package processor

import (
	"time"

	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrtrace"
	processconsume "github.com/mondegor/go-webcore/mrworker/process/consume"

	"github.com/mondegor/go-components/mrmailer/infra/handler"
	"github.com/mondegor/go-components/mrmailer/repository"
	"github.com/mondegor/go-components/mrmailer/sendmessage/provider"
	"github.com/mondegor/go-components/mrmailer/service/consume"
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
) *processconsume.MessageProcessor {
	o := options{
		processorOpts: []processconsume.Option{
			processconsume.WithCaptionPrefix(defaultCaptionPrefix),
			processconsume.WithReadyTimeout(defaultReadyTimeout),
			processconsume.WithReadPeriod(defaultReadPeriod),
			processconsume.WithConsumerTimeout(defaultConsumerReadTimeout, defaultConsumerWriteTimeout),
			processconsume.WithHandlerTimeout(defaultHandlerTimeout),
			processconsume.WithQueueSize(defaultQueueSize),
			processconsume.WithWorkersCount(defaultWorkersCount),
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

	messageConsumer := consume.New(
		client,
		storageMessage,
		queueconsume.New(
			client,
			storageQueue,
			queueconsume.WithStorageCompleted(storageQueueCompleted),
			queueconsume.WithStorageCrashed(storageQueueCrashed),
		),
	)

	return processconsume.NewMessageProcessor(
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
