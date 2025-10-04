package scheduler

import (
	"context"
	"time"

	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/errorwrapper"
	"github.com/mondegor/go-sysmess/mrevent"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrtrace"
	"github.com/mondegor/go-webcore/mrworker"
	"github.com/mondegor/go-webcore/mrworker/job/task"
	"github.com/mondegor/go-webcore/mrworker/process/schedule"

	"github.com/mondegor/go-components/mrnotifier/notifier/entity"
	"github.com/mondegor/go-components/mrnotifier/notifier/repository"
	"github.com/mondegor/go-components/mrnotifier/notifier/usecase/change"
	"github.com/mondegor/go-components/mrnotifier/notifier/usecase/clean"
	queuerepository "github.com/mondegor/go-components/mrqueue/repository"
	queuechange "github.com/mondegor/go-components/mrqueue/usecase/change"
	queueclean "github.com/mondegor/go-components/mrqueue/usecase/clean"
)

const (
	defaultCaptionPrefix      = "Notifier"
	defaultChangeLimit        = 100
	defaultChangeRetryTimeout = 60 * time.Second
	defaultChangeRetryDelayed = 30 * time.Second
	defaultCleanLimit         = 100

	defaultChangeFromToRetryCaption = "Task/ChangeFromToRetry"
	defaultChangeFromToRetryPeriod  = 90 * time.Second
	defaultChangeFromToRetryTimeout = 15 * time.Second

	defaultCleanNoticesCaption = "Task/CleanNotices"
	defaultCleanNoticesPeriod  = 45 * time.Minute
	defaultCleanNoticesTimeout = 120 * time.Second
)

type (
	serviceOptions struct {
		captionPrefix         string
		changeLimit           int
		changeRetryTimeout    time.Duration
		changeRetryDelayed    time.Duration
		cleanLimit            int
		taskChangeFromToRetry []task.Option
		taskCleanNotices      []task.Option
	}
)

// NewService - создаёт сервис для обработки уведомлений и связанных с ним задачи.
func NewService(
	client mrstorage.DBConnManager,
	eventEmitter mrevent.Emitter,
	errorHandler mrerr.ErrorHandler,
	logger mrlog.Logger,
	traceManager mrtrace.ContextManager,
	noticeTable mrsql.DBTableInfo,
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
		taskCleanNotices: []task.Option{
			task.WithCaptionPrefix(defaultCaptionPrefix),
			task.WithCaption(defaultCleanNoticesCaption),
			task.WithPeriod(defaultCleanNoticesPeriod),
			task.WithTimeout(defaultCleanNoticesTimeout),
		},
	}

	for _, opt := range opts {
		opt(&o)
	}

	storageNotice := repository.NewNoticePostgres(client, noticeTable)

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

	eventEmitterQueue := mrevent.NewSourceEmitter(eventEmitter, entity.ModelNameNotice)

	noticeCleaner := clean.New(
		client,
		storageNotice,
		queueclean.New(
			storageQueue,
			eventEmitterQueue,
			errorwrapper.NewUseCase(),
			queueclean.WithStorageCompleted(storageQueueCompleted),
			queueclean.WithStorageBroken(storageQueueBroken),
		),
		errorwrapper.NewUseCase(),
	)

	statusChanger := change.New(
		queuechange.New(
			client,
			storageQueue,
			eventEmitterQueue,
			errorwrapper.NewUseCase(),
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
			if err := noticeCleaner.RemoveNoticesWithoutAttempts(ctx, o.cleanLimit); err != nil {
				return err
			}

			if err := noticeCleaner.RemoveCompletedNotices(ctx, o.cleanLimit); err != nil {
				return err
			}

			return noticeCleaner.RemoveBrokenNotices(ctx, o.cleanLimit)
		}),
		o.taskCleanNotices...,
	)

	return schedule.NewTaskScheduler(
		errorHandler,
		logger,
		traceManager,
		schedule.WithCaptionPrefix(o.captionPrefix),
		schedule.WithTasks(changerTask, cleanerTask),
	)
}
