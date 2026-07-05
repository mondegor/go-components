package scheduler

import (
	"context"
	"time"

	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrevent"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrprocess"
	"github.com/mondegor/go-sysmess/mrprocess/job/task"
	"github.com/mondegor/go-sysmess/mrprocess/schedule"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-sysmess/mrstorage/mrsql"
	"github.com/mondegor/go-sysmess/mrtrace"

	"github.com/mondegor/go-components/mrmailer/entity"
	"github.com/mondegor/go-components/mrmailer/repository"
	queuerepository "github.com/mondegor/go-components/mrqueue/repository"
	queuetoreadychange "github.com/mondegor/go-components/mrqueue/usecase/change/toready"
	queuetoretrychange "github.com/mondegor/go-components/mrqueue/usecase/change/toretry"
	queuecompletedclean "github.com/mondegor/go-components/mrqueue/usecase/completed/clean"
	queuecrashedclean "github.com/mondegor/go-components/mrqueue/usecase/crashed/clean"
	"github.com/mondegor/go-components/wire/mrqueue/change"
	"github.com/mondegor/go-components/wire/mrqueue/clean"
)

const (
	defaultCaptionPrefix      = "Mailer"
	defaultChangeBatchSize    = 100
	defaultChangeRetryTimeout = 60 * time.Second
	defaultChangeRetryDelayed = 30 * time.Second
	defaultCleanBatchSize     = 100

	defaultChangeFromToRetryCaption = "Task/ChangeFromToRetry"
	defaultChangeFromToRetryPeriod  = 90 * time.Second
	defaultChangeFromToRetryTimeout = 15 * time.Second

	defaultCleanMessagesCaption = "Task/CleanQueue"
	defaultCleanMessagesPeriod  = 45 * time.Minute
	defaultCleanMessagesTimeout = 120 * time.Second
)

// InitService - создаёт сервис для обработки и отправки сообщений и связанных с ним задачи.
func InitService(
	client mrstorage.DBConnManager,
	eventEmitter mrevent.Emitter,
	errorHandler errors.Handler,
	logger mrlog.Logger,
	traceManager mrtrace.ContextManager,
	messageTable mrsql.DBTableInfo,
	queueTable mrsql.DBTableInfo,
	opts ...Option,
) *schedule.TaskScheduler {
	o := options{
		captionPrefix:      defaultCaptionPrefix,
		changeRetryTimeout: defaultChangeRetryTimeout,
		changeRetryDelayed: defaultChangeRetryDelayed,
		taskChangerOpts: []task.Option{
			task.WithCaptionPrefix(defaultCaptionPrefix),
			task.WithCaption(defaultChangeFromToRetryCaption),
			task.WithPeriod(defaultChangeFromToRetryPeriod),
			task.WithTimeout(defaultChangeFromToRetryTimeout),
		},
		taskCleanerOpts: []task.Option{
			task.WithCaptionPrefix(defaultCaptionPrefix),
			task.WithCaption(defaultCleanMessagesCaption),
			task.WithPeriod(defaultCleanMessagesPeriod),
			task.WithTimeout(defaultCleanMessagesTimeout),
		},
	}

	for _, opt := range opts {
		opt(&o)
	}

	if o.changeBatchSize < 1 {
		o.changeBatchSize = defaultChangeBatchSize
	}

	if o.cleanBatchSize < 1 {
		o.cleanBatchSize = defaultCleanBatchSize
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

	queueEventEmitter := mrevent.EmitterWithSource(eventEmitter, entity.ModelNameMessage)

	forgottenMessageCleaner := clean.InitForgottenItemsCleaner(
		client,
		storageQueue,
		queueEventEmitter,
	)

	completedMessageCleaner := clean.InitCompletedItemsCleaner(
		client,
		storageQueueCompleted,
		queueEventEmitter,
		queuecompletedclean.WithAfterClean(func(ctx context.Context, itemsIDs []uint64) error {
			return storageMessage.DeleteByIDs(ctx, itemsIDs)
		}),
	)

	crashedMessageCleaner := clean.InitCrashedItemsCleaner(
		client,
		storageQueueCrashed,
		queueEventEmitter,
		queuecrashedclean.WithAfterClean(func(ctx context.Context, itemsIDs []uint64) error {
			return storageMessage.DeleteByIDs(ctx, itemsIDs)
		}),
	)

	messageStatusToReadyChanger := change.InitRetryToReadyChanger(
		storageQueue,
		queueEventEmitter,
		queuetoreadychange.WithRetryDelayed(o.changeRetryDelayed),
	)

	messageStatusToRetryChanger := change.InitProcessingToRetryChanger(
		client,
		storageQueue,
		queueEventEmitter,
		queuetoretrychange.WithStorageCrashed(storageQueueCrashed),
		queuetoretrychange.WithRetryTimeout(o.changeRetryTimeout),
	)

	changerTask := task.NewJobWrapper(
		mrprocess.JobFunc(func(ctx context.Context) error {
			if err := messageStatusToReadyChanger.Execute(ctx, o.changeBatchSize); err != nil {
				return err
			}

			return messageStatusToRetryChanger.Execute(ctx, o.changeBatchSize)
		}),
		o.taskChangerOpts...,
	)

	cleanerTask := task.NewJobWrapper(
		mrprocess.JobFunc(func(ctx context.Context) error {
			if err := forgottenMessageCleaner.Execute(ctx, o.cleanBatchSize); err != nil {
				return err
			}

			if err := completedMessageCleaner.Execute(ctx, o.cleanBatchSize); err != nil {
				return err
			}

			return crashedMessageCleaner.Execute(ctx, o.cleanBatchSize)
		}),
		o.taskCleanerOpts...,
	)

	return schedule.NewTaskScheduler(
		errorHandler,
		logger,
		traceManager,
		schedule.WithCaptionPrefix(o.captionPrefix),
		schedule.WithTasks(changerTask, cleanerTask),
	)
}
