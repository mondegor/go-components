package scheduler

import (
	"context"
	"time"

	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrevent"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrtrace"
	"github.com/mondegor/go-webcore/mrworker"
	"github.com/mondegor/go-webcore/mrworker/job/task"
	"github.com/mondegor/go-webcore/mrworker/process/schedule"

	"github.com/mondegor/go-components/mrnotifier/notifier/entity"
	"github.com/mondegor/go-components/mrnotifier/notifier/repository"
	queuerepository "github.com/mondegor/go-components/mrqueue/repository"
	queuetoreadychange "github.com/mondegor/go-components/mrqueue/usecase/change/toready"
	queuetoretrychange "github.com/mondegor/go-components/mrqueue/usecase/change/toretry"
	queuecompletedclean "github.com/mondegor/go-components/mrqueue/usecase/completed/clean"
	queuecrashedclean "github.com/mondegor/go-components/mrqueue/usecase/crashed/clean"
	"github.com/mondegor/go-components/wire/mrqueue/change"
	"github.com/mondegor/go-components/wire/mrqueue/clean"
)

const (
	defaultCaptionPrefix      = "Notifier"
	defaultChangeBatchSize    = 100
	defaultChangeRetryTimeout = 60 * time.Second
	defaultChangeRetryDelayed = 30 * time.Second
	defaultCleanBatchSize     = 100

	defaultChangeFromToRetryCaption = "Task/ChangeFromToRetry"
	defaultChangeFromToRetryPeriod  = 90 * time.Second
	defaultChangeFromToRetryTimeout = 15 * time.Second

	defaultCleanNoticesCaption = "Task/CleanNotices"
	defaultCleanNoticesPeriod  = 45 * time.Minute
	defaultCleanNoticesTimeout = 120 * time.Second
)

// InitService - создаёт сервис для обработки уведомлений и связанных с ним задачи.
func InitService(
	client mrstorage.DBConnManager,
	eventEmitter mrevent.Emitter,
	errorHandler errors.Handler,
	logger mrlog.Logger,
	traceManager mrtrace.ContextManager,
	noticeTable mrsql.DBTableInfo,
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
			task.WithCaption(defaultCleanNoticesCaption),
			task.WithPeriod(defaultCleanNoticesPeriod),
			task.WithTimeout(defaultCleanNoticesTimeout),
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

	storageNotice := repository.NewNotePostgres(client, noticeTable)

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

	queueEventEmitter := mrevent.EmitterWithSource(eventEmitter, entity.ModelNameNotice)

	forgottenNoticeCleaner := clean.InitForgottenItemsCleaner(
		client,
		storageQueue,
		queueEventEmitter,
	)

	completedNoticeCleaner := clean.InitCompletedItemsCleaner(
		client,
		storageQueueCompleted,
		queueEventEmitter,
		queuecompletedclean.WithAfterClean(func(ctx context.Context, itemsIDs []uint64) error {
			return storageNotice.DeleteByIDs(ctx, itemsIDs)
		}),
	)

	crashedNoticeCleaner := clean.InitCrashedItemsCleaner(
		client,
		storageQueueCrashed,
		queueEventEmitter,
		queuecrashedclean.WithAfterClean(func(ctx context.Context, itemsIDs []uint64) error {
			return storageNotice.DeleteByIDs(ctx, itemsIDs)
		}),
	)

	noticeStatusToReadyChanger := change.InitRetryToReadyChanger(
		storageQueue,
		queueEventEmitter,
		queuetoreadychange.WithRetryDelayed(o.changeRetryDelayed),
	)

	noticeStatusToRetryChanger := change.InitProcessingToRetryChanger(
		client,
		storageQueue,
		queueEventEmitter,
		queuetoretrychange.WithStorageCrashed(storageQueueCrashed),
		queuetoretrychange.WithRetryTimeout(o.changeRetryTimeout),
	)

	changerTask := task.NewJobWrapper(
		mrworker.JobFunc(func(ctx context.Context) error {
			if err := noticeStatusToReadyChanger.Execute(ctx, o.changeBatchSize); err != nil {
				return err
			}

			return noticeStatusToRetryChanger.Execute(ctx, o.changeBatchSize)
		}),
		o.taskChangerOpts...,
	)

	cleanerTask := task.NewJobWrapper(
		mrworker.JobFunc(func(ctx context.Context) error {
			if err := forgottenNoticeCleaner.Execute(ctx, o.cleanBatchSize); err != nil {
				return err
			}

			if err := completedNoticeCleaner.Execute(ctx, o.cleanBatchSize); err != nil {
				return err
			}

			return crashedNoticeCleaner.Execute(ctx, o.cleanBatchSize)
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
