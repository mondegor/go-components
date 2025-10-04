package processor

import (
	"time"

	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrevent"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrtrace"
	processconsume "github.com/mondegor/go-webcore/mrworker/process/consume"

	"github.com/mondegor/go-components/mrnotifier"
	"github.com/mondegor/go-components/mrnotifier/notifier/entity"
	"github.com/mondegor/go-components/mrnotifier/notifier/repository"
	"github.com/mondegor/go-components/mrnotifier/notifier/usecase/build"
	"github.com/mondegor/go-components/mrnotifier/notifier/usecase/consume"
	"github.com/mondegor/go-components/mrnotifier/notifier/usecase/handle"
	templaterepository "github.com/mondegor/go-components/mrnotifier/template/repository"
	templateusecase "github.com/mondegor/go-components/mrnotifier/template/usecase"
	queuerepository "github.com/mondegor/go-components/mrqueue/repository"
	queueconsume "github.com/mondegor/go-components/mrqueue/usecase/consume"
)

const (
	defaultDefaultLang = "en-US"

	defaultCaptionPrefix        = "Notifier"
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
		defaultLang     string
		noticeProcessor []processconsume.Option
	}
)

// NewService - создаёт сервис для обработки уведомлений и связанных с ним задачи.
func NewService(
	client mrstorage.DBConnManager,
	mailerAPI mrnotifier.MailerAPI,
	eventEmitter mrevent.Emitter,
	errorHandler mrerr.ErrorHandler,
	useCaseErrorWrapper mrerr.UseCaseErrorWrapper,
	storageErrorWrapper mrerr.ErrorWrapper,
	logger mrlog.Logger,
	traceManager mrtrace.ContextManager,
	noticeTable mrsql.DBTableInfo,
	queueTable mrsql.DBTableInfo,
	templateTableName string,
	templateVarName string,
	opts ...ServiceOption,
) *processconsume.MessageProcessor {
	o := serviceOptions{
		defaultLang: defaultDefaultLang,
		noticeProcessor: []processconsume.Option{
			processconsume.WithCaptionPrefix(defaultCaptionPrefix),
			processconsume.WithReadyTimeout(defaultReadyTimeout),
			processconsume.WithReadPeriod(defaultReadPeriod),
			processconsume.WithConsumerTimeout(defaultConsumerReadTimeout, defaultConsumerWriteTimeout),
			processconsume.WithHandlerTimeout(defaultHandlerTimeout),
			processconsume.WithQueueSize(defaultQueueSize),
			processconsume.WithWorkersCount(defaultWorkersCount),
		},
	}

	for _, opt := range opts {
		opt(&o)
	}

	storageNotice := repository.NewNoticePostgres(client, noticeTable)

	storageTemplate := templaterepository.NewTemplatePostgres(
		client,
		storageErrorWrapper,
		logger,
		templateTableName,
		templateVarName,
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

	eventEmitterQueue := mrevent.NewSourceEmitter(eventEmitter, entity.ModelNameNotice)

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
	)

	return processconsume.NewMessageProcessor(
		noticeConsumer,
		handle.New(
			build.New(
				templateusecase.New(
					storageTemplate,
					useCaseErrorWrapper,
					logger,
					o.defaultLang,
				),
			),
			mailerAPI,
		),
		errorHandler,
		logger,
		traceManager,
		o.noticeProcessor...,
	)
}
