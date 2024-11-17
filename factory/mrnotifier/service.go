package mrnotifier

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

	"github.com/mondegor/go-components/mrnotifier"
	"github.com/mondegor/go-components/mrnotifier/notifier/component/build"
	"github.com/mondegor/go-components/mrnotifier/notifier/component/change"
	"github.com/mondegor/go-components/mrnotifier/notifier/component/clean"
	"github.com/mondegor/go-components/mrnotifier/notifier/component/consume"
	"github.com/mondegor/go-components/mrnotifier/notifier/component/handle"
	"github.com/mondegor/go-components/mrnotifier/notifier/entity"
	"github.com/mondegor/go-components/mrnotifier/notifier/repository"
	templaterepository "github.com/mondegor/go-components/mrnotifier/template/repository"
	templateusecase "github.com/mondegor/go-components/mrnotifier/template/usecase"
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
	defaultDefaultLang        = "en_EN"

	defaultSendProcessorCaption           = "Notifier.SendProcessor"
	defaultSendProcessorReadyTimeout      = 60 * time.Second
	defaultSendProcessorReadPeriod        = 30 * time.Second
	defaultSendProcessorBusyReadPeriod    = 15 * time.Second
	defaultSendProcessorCancelReadTimeout = 5 * time.Second
	defaultSendProcessorHandlerTimeout    = 30 * time.Second
	defaultSendProcessorQueueSize         = 25
	defaultSendProcessorWorkersCount      = 1

	defaultChangeFromToRetryCaption = "Notifier.ChangeFromToRetry"
	defaultChangeFromToRetryPeriod  = 90 * time.Second
	defaultChangeFromToRetryTimeout = 15 * time.Second

	defaultCleanQueueCaption = "Notifier.CleanQueue"
	defaultCleanQueuePeriod  = 45 * time.Minute
	defaultCleanQueueTimeout = 120 * time.Second
)

type (
	serviceOptions struct {
		changeLimit           uint32
		changeRetryTimeout    time.Duration
		changeRetryDelayed    time.Duration
		cleanLimit            uint32
		defaultLang           string
		sendProcessor         []processconsume.Option
		taskChangeFromToRetry []task.Option
		taskCleanNotices      []task.Option
	}
)

// NewComponentService - создаёт сервис для обработки уведомлений и связанных с ним задачи.
func NewComponentService(
	client mrstorage.DBConnManager,
	mailerAPI mrnotifier.MailerAPI,
	eventEmitter mrsender.EventEmitter,
	errorHandler mrcore.ErrorHandler,
	noticeTable mrsql.DBTableInfo,
	queueTable mrsql.DBTableInfo,
	templateTableName string,
	templateVarName string,
	opts ...ServiceOption,
) (mrrun.Process, []mrworker.Task) {
	options := serviceOptions{
		changeLimit:        defaultChangeLimit,
		changeRetryTimeout: defaultChangeRetryTimeout,
		changeRetryDelayed: defaultChangeRetryDelayed,
		cleanLimit:         defaultCleanLimit,
		defaultLang:        defaultDefaultLang,
		sendProcessor: []processconsume.Option{
			processconsume.WithCaption(defaultSendProcessorCaption),
			processconsume.WithReadyTimeout(defaultSendProcessorReadyTimeout),
			processconsume.WithReadPeriod(defaultSendProcessorReadPeriod),
			processconsume.WithBusyReadPeriod(defaultSendProcessorBusyReadPeriod),
			processconsume.WithCancelReadTimeout(defaultSendProcessorCancelReadTimeout),
			processconsume.WithHandlerTimeout(defaultSendProcessorHandlerTimeout),
			processconsume.WithQueueSize(defaultSendProcessorQueueSize),
			processconsume.WithWorkersCount(defaultSendProcessorWorkersCount),
		},
		taskChangeFromToRetry: []task.Option{
			task.WithCaption(defaultChangeFromToRetryCaption),
			task.WithPeriod(defaultChangeFromToRetryPeriod),
			task.WithTimeout(defaultChangeFromToRetryTimeout),
		},
		taskCleanNotices: []task.Option{
			task.WithCaption(defaultCleanQueueCaption),
			task.WithPeriod(defaultCleanQueuePeriod),
			task.WithTimeout(defaultCleanQueueTimeout),
		},
	}

	for _, opt := range opts {
		opt(&options)
	}

	storageNotice := repository.NewNoticePostgres(client, noticeTable)

	storageTemplate := templaterepository.NewTemplatePostgres(
		client,
		templateTableName,
		templateVarName,
		mrapp.NewStorageErrorWrapper(),
	)

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

	eventEmitterQueue := decorator.NewSourceEmitter(eventEmitter, entity.ModelNameNotice)
	useCaseErrorWrapper := mrapp.NewUseCaseErrorWrapper()

	noticeConsumer := consume.New(
		client,
		storageNotice,
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
		noticeConsumer,
		handle.New(
			build.New(
				templateusecase.New(
					storageTemplate,
					useCaseErrorWrapper,
					options.defaultLang,
				),
			),
			mailerAPI,
		),
		errorHandler,
		options.sendProcessor...,
	)

	noticeCleaner := clean.New(
		client,
		storageNotice,
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
			if err := noticeCleaner.RemoveNoticesWithoutAttempts(ctx, options.cleanLimit); err != nil {
				return err
			}

			if err := noticeCleaner.RemoveCompletedNotices(ctx, options.cleanLimit); err != nil {
				return err
			}

			return noticeCleaner.RemoveBrokenNotices(ctx, options.cleanLimit)
		}),
		options.taskCleanNotices...,
	)

	return processor, []mrworker.Task{changerJobTask, cleanerJobTask}
}
