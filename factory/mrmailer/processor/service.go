package processor

import (
	"time"

	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrtrace"
	"github.com/mondegor/go-webcore/mrsender"
	"github.com/mondegor/go-webcore/mrsender/decorator"
	processconsume "github.com/mondegor/go-webcore/mrworker/process/consume"

	core "github.com/mondegor/go-components/internal"
	"github.com/mondegor/go-components/mrmailer/entity"
	"github.com/mondegor/go-components/mrmailer/repository"
	"github.com/mondegor/go-components/mrmailer/usecase/consume"
	"github.com/mondegor/go-components/mrmailer/usecase/handle"
	queuerepository "github.com/mondegor/go-components/mrqueue/repository"
	queueconsume "github.com/mondegor/go-components/mrqueue/usecase/consume"
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

type (
	serviceOptions struct {
		messageProcessor []processconsume.Option
		messageHandler   []handle.Option
	}
)

// NewService - создаёт сервис для обработки и отправки сообщений и связанных с ним задачи.
func NewService(
	client mrstorage.DBConnManager,
	eventEmitter mrsender.EventEmitter,
	errorHandler core.ErrorHandler,
	logger mrlog.Logger,
	trace mrtrace.Tracer,
	traceManager core.TraceManager,
	messageTable mrsql.DBTableInfo,
	queueTable mrsql.DBTableInfo,
	opts ...ServiceOption,
) *processconsume.MessageProcessor {
	o := serviceOptions{
		messageProcessor: []processconsume.Option{
			processconsume.WithCaptionPrefix(defaultCaptionPrefix),
			processconsume.WithReadyTimeout(defaultReadyTimeout),
			processconsume.WithReadPeriod(defaultReadPeriod),
			processconsume.WithConsumerTimeout(defaultConsumerReadTimeout, defaultConsumerWriteTimeout),
			processconsume.WithHandlerTimeout(defaultHandlerTimeout),
			processconsume.WithQueueSize(defaultQueueSize),
			processconsume.WithWorkersCount(defaultWorkersCount),
		},
		messageHandler: nil,
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
	storageQueueBroken := queuerepository.NewBrokenPostgres(
		client,
		mrsql.DBTableInfo{
			Name:       queueTable.Name + "_errors",
			PrimaryKey: queueTable.PrimaryKey,
		},
	)

	eventEmitterQueue := decorator.NewSourceEmitter(eventEmitter, entity.ModelNameMessage)

	messageConsumer := consume.New(
		client,
		storageMessage,
		queueconsume.New(
			client,
			storageQueue,
			eventEmitterQueue,
			queueconsume.WithStorageCompleted(storageQueueCompleted),
			queueconsume.WithStorageBroken(storageQueueBroken),
		),
	)

	return processconsume.NewMessageProcessor(
		messageConsumer,
		handle.New(
			trace,
			o.messageHandler...,
		),
		errorHandler,
		logger,
		traceManager,
		o.messageProcessor...,
	)
}
