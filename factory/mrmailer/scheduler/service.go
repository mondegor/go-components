package scheduler

import (
	"context"
	"time"

	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-webcore/mrsender"
	"github.com/mondegor/go-webcore/mrsender/decorator"
	"github.com/mondegor/go-webcore/mrworker"
	"github.com/mondegor/go-webcore/mrworker/job/task"
	"github.com/mondegor/go-webcore/mrworker/process/schedule"

	core "github.com/mondegor/go-components/internal"
	"github.com/mondegor/go-components/mrmailer/entity"
	"github.com/mondegor/go-components/mrmailer/repository"
	"github.com/mondegor/go-components/mrmailer/usecase/change"
	"github.com/mondegor/go-components/mrmailer/usecase/clean"
	queuerepository "github.com/mondegor/go-components/mrqueue/repository"
	queuechange "github.com/mondegor/go-components/mrqueue/usecase/change"
	queueclean "github.com/mondegor/go-components/mrqueue/usecase/clean"
)

const (
	defaultCaptionPrefix      = "Mailer"
	defaultChangeLimit        = 100
	defaultChangeRetryTimeout = 60 * time.Second
	defaultChangeRetryDelayed = 30 * time.Second
	defaultCleanLimit         = 100

	defaultChangeFromToRetryCaption = "Task/ChangeFromToRetry"
	defaultChangeFromToRetryPeriod  = 90 * time.Second
	defaultChangeFromToRetryTimeout = 15 * time.Second

	defaultCleanMessagesCaption = "Task/CleanQueue"
	defaultCleanMessagesPeriod  = 45 * time.Minute
	defaultCleanMessagesTimeout = 120 * time.Second
)

type (
	serviceOptions struct {
		captionPrefix         string
		changeLimit           int
		changeRetryTimeout    time.Duration
		changeRetryDelayed    time.Duration
		cleanLimit            int
		taskChangeFromToRetry []task.Option
		taskCleanMessages     []task.Option
	}
)

// NewService - создаёт сервис для обработки и отправки сообщений и связанных с ним задачи.
func NewService(
	client mrstorage.DBConnManager,
	eventEmitter mrsender.EventEmitter,
	errorHandler core.ErrorHandler,
	logger mrlog.Logger,
	traceManager core.TraceManager,
	messageTable mrsql.DBTableInfo,
	queueTable mrsql.DBTableInfo,
	opts ...ServiceOption,
) *schedule.TaskScheduler {
	o := serviceOptions{
		captionPrefix:      defaultCaptionPrefix,
		changeLimit:        defaultChangeLimit,
		changeRetryTimeout: defaultChangeRetryTimeout,
		changeRetryDelayed: defaultChangeRetryDelayed,
		cleanLimit:         defaultCleanLimit,
		taskChangeFromToRetry: []task.Option{
			task.WithCaptionPrefix(defaultCaptionPrefix),
			task.WithCaption(defaultChangeFromToRetryCaption),
			task.WithPeriod(defaultChangeFromToRetryPeriod),
			task.WithTimeout(defaultChangeFromToRetryTimeout),
		},
		taskCleanMessages: []task.Option{
			task.WithCaptionPrefix(defaultCaptionPrefix),
			task.WithCaption(defaultCleanMessagesCaption),
			task.WithPeriod(defaultCleanMessagesPeriod),
			task.WithTimeout(defaultCleanMessagesTimeout),
		},
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

	messageCleaner := clean.New(
		client,
		storageMessage,
		queueclean.New(
			storageQueue,
			eventEmitterQueue,
			queueclean.WithStorageCompleted(storageQueueCompleted),
			queueclean.WithStorageBroken(storageQueueBroken),
		),
	)

	statusChanger := change.New(
		queuechange.New(
			client,
			storageQueue,
			eventEmitterQueue,
			queuechange.WithStorageBroken(storageQueueBroken),
			queuechange.WithRetryTimeout(o.changeRetryTimeout),
			queuechange.WithRetryDelayed(o.changeRetryDelayed),
		),
	)

	changerTask := task.NewJobWrapper(
		mrworker.JobFunc(func(ctx context.Context) error {
			if err := statusChanger.ChangeProcessingToRetryByTimeout(ctx, o.cleanLimit); err != nil {
				return err
			}

			return statusChanger.ChangeRetryToReady(ctx, o.changeLimit)
		}),
		o.taskChangeFromToRetry...,
	)

	cleanerTask := task.NewJobWrapper(
		mrworker.JobFunc(func(ctx context.Context) error {
			if err := messageCleaner.RemoveMessagesWithoutAttempts(ctx, o.cleanLimit); err != nil {
				return err
			}

			if err := messageCleaner.RemoveCompletedMessages(ctx, o.cleanLimit); err != nil {
				return err
			}

			return messageCleaner.RemoveBrokenMessages(ctx, o.cleanLimit)
		}),
		o.taskCleanMessages...,
	)

	return schedule.NewTaskScheduler(
		errorHandler,
		logger,
		traceManager,
		schedule.WithCaptionPrefix(o.captionPrefix),
		schedule.WithTasks(changerTask, cleanerTask),
	)
}
