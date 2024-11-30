package mrmailer

import (
	"context"
	"time"

	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrcore/mrapp"
	"github.com/mondegor/go-webcore/mrrun"
	"github.com/mondegor/go-webcore/mrsender"
	"github.com/mondegor/go-webcore/mrsender/decorator"
	"github.com/mondegor/go-webcore/mrworker"
	"github.com/mondegor/go-webcore/mrworker/job/task"
	processconsume "github.com/mondegor/go-webcore/mrworker/process/consume"

	"github.com/mondegor/go-components/mrmailer/component/change"
	"github.com/mondegor/go-components/mrmailer/component/clean"
	"github.com/mondegor/go-components/mrmailer/component/consume"
	"github.com/mondegor/go-components/mrmailer/component/handle"
	"github.com/mondegor/go-components/mrmailer/entity"
	"github.com/mondegor/go-components/mrmailer/repository"
	queuechange "github.com/mondegor/go-components/mrqueue/component/change"
	queueclean "github.com/mondegor/go-components/mrqueue/component/clean"
	queueconsume "github.com/mondegor/go-components/mrqueue/component/consume"
	queuerepository "github.com/mondegor/go-components/mrqueue/repository"
)

const (
	defaultChangeLimit        = 100
	defaultChangeRetryTimeout = 60 * time.Second
	defaultChangeRetryDelayed = 30 * time.Second
	defaultCleanLimit         = 100

	defaultSendProcessorCaption           = "Mailer.SendProcessor"
	defaultSendProcessorReadyTimeout      = 60 * time.Second
	defaultSendProcessorStartReadDelay    = 0 * time.Second
	defaultSendProcessorReadPeriod        = 30 * time.Second
	defaultSendProcessorCancelReadTimeout = 5 * time.Second
	defaultSendProcessorHandlerTimeout    = 30 * time.Second
	defaultSendProcessorQueueSize         = 25
	defaultSendProcessorWorkersCount      = 1

	defaultChangeFromToRetryCaption = "Mailer.ChangeFromToRetry"
	defaultChangeFromToRetryPeriod  = 90 * time.Second
	defaultChangeFromToRetryTimeout = 15 * time.Second

	defaultCleanQueueCaption = "Mailer.CleanQueue"
	defaultCleanQueuePeriod  = 45 * time.Minute
	defaultCleanQueueTimeout = 120 * time.Second
)

type (
	serviceOptions struct {
		changeLimit           uint32
		changeRetryTimeout    time.Duration
		changeRetryDelayed    time.Duration
		cleanLimit            uint32
		sendProcessor         []processconsume.Option
		sendHandler           []handle.Option
		taskChangeFromToRetry []task.Option
		taskCleanMessages     []task.Option
	}
)

// NewComponentService - создаёт сервис для обработки и отправки сообщений и связанных с ним задачи.
func NewComponentService(
	client mrstorage.DBConnManager,
	eventEmitter mrsender.EventEmitter,
	errorHandler mrcore.ErrorHandler,
	messageTable mrsql.DBTableInfo,
	queueTable mrsql.DBTableInfo,
	opts ...ServiceOption,
) (mrrun.Process, []mrworker.Task) {
	options := serviceOptions{
		changeLimit:        defaultChangeLimit,
		changeRetryTimeout: defaultChangeRetryTimeout,
		changeRetryDelayed: defaultChangeRetryDelayed,
		cleanLimit:         defaultCleanLimit,
		sendProcessor: []processconsume.Option{
			processconsume.WithCaption(defaultSendProcessorCaption),
			processconsume.WithReadyTimeout(defaultSendProcessorReadyTimeout),
			processconsume.WithStartReadDelay(defaultSendProcessorStartReadDelay),
			processconsume.WithReadPeriod(defaultSendProcessorReadPeriod),
			processconsume.WithCancelReadTimeout(defaultSendProcessorCancelReadTimeout),
			processconsume.WithHandlerTimeout(defaultSendProcessorHandlerTimeout),
			processconsume.WithQueueSize(defaultSendProcessorQueueSize),
			processconsume.WithWorkersCount(defaultSendProcessorWorkersCount),
		},
		sendHandler: nil,
		taskChangeFromToRetry: []task.Option{
			task.WithCaption(defaultChangeFromToRetryCaption),
			task.WithPeriod(defaultChangeFromToRetryPeriod),
			task.WithTimeout(defaultChangeFromToRetryTimeout),
		},
		taskCleanMessages: []task.Option{
			task.WithCaption(defaultCleanQueueCaption),
			task.WithPeriod(defaultCleanQueuePeriod),
			task.WithTimeout(defaultCleanQueueTimeout),
		},
	}

	for _, opt := range opts {
		opt(&options)
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
	useCaseErrorWrapper := mrapp.NewUseCaseErrorWrapper()

	messageConsumer := consume.New(
		client,
		storageMessage,
		queueconsume.New(
			client,
			storageQueue,
			eventEmitterQueue,
			useCaseErrorWrapper,
			queueconsume.WithStorageCompleted(storageQueueCompleted),
			queueconsume.WithStorageBroken(storageQueueBroken),
		),
		useCaseErrorWrapper,
	)

	processor := processconsume.NewMessageProcessor(
		messageConsumer,
		handle.New(
			useCaseErrorWrapper,
			options.sendHandler...,
		),
		errorHandler,
		options.sendProcessor...,
	)

	messageCleaner := clean.New(
		client,
		storageMessage,
		queueclean.New(
			storageQueue,
			eventEmitterQueue,
			useCaseErrorWrapper,
			queueclean.WithStorageCompleted(storageQueueCompleted),
			queueclean.WithStorageBroken(storageQueueBroken),
		),
		useCaseErrorWrapper,
	)

	statusChanger := change.New(
		queuechange.New(
			client,
			storageQueue,
			eventEmitterQueue,
			useCaseErrorWrapper,
			queuechange.WithStorageBroken(storageQueueBroken),
			queuechange.WithRetryTimeout(options.changeRetryTimeout),
			queuechange.WithRetryDelayed(options.changeRetryDelayed),
		),
	)

	changerJobTask := task.NewJobWrapper(
		mrworker.JobFunc(func(ctx context.Context) error {
			if err := statusChanger.ChangeProcessingToRetryByTimeout(ctx, options.cleanLimit); err != nil {
				return err
			}

			return statusChanger.ChangeRetryToReady(ctx, options.changeLimit)
		}),
		options.taskChangeFromToRetry...,
	)

	cleanerJobTask := task.NewJobWrapper(
		mrworker.JobFunc(func(ctx context.Context) error {
			if err := messageCleaner.RemoveMessagesWithoutAttempts(ctx, options.cleanLimit); err != nil {
				return err
			}

			if err := messageCleaner.RemoveCompletedMessages(ctx, options.cleanLimit); err != nil {
				return err
			}

			return messageCleaner.RemoveBrokenMessages(ctx, options.cleanLimit)
		}),
		options.taskCleanMessages...,
	)

	return processor, []mrworker.Task{changerJobTask, cleanerJobTask}
}
