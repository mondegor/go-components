package processor

import (
	"time"

	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrtrace"
	processconsume "github.com/mondegor/go-webcore/mrworker/process/consume"

	"github.com/mondegor/go-components/mrnotifier"
	"github.com/mondegor/go-components/mrnotifier/notifier/infra/handler"
	"github.com/mondegor/go-components/mrnotifier/notifier/repository"
	"github.com/mondegor/go-components/mrnotifier/notifier/service/consume"
	"github.com/mondegor/go-components/mrnotifier/notifier/usecase"
	templaterepository "github.com/mondegor/go-components/mrnotifier/template/repository"
	templateservice "github.com/mondegor/go-components/mrnotifier/template/service"
	queuerepository "github.com/mondegor/go-components/mrqueue/repository"
	queueconsume "github.com/mondegor/go-components/mrqueue/service/consume"
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

// InitService - создаёт сервис для обработки уведомлений и связанных с ним задачи.
func InitService(
	client mrstorage.DBConnManager,
	noticeProvider mrnotifier.NoticeSender,
	errorHandler errors.Handler,
	logger mrlog.Logger,
	traceManager mrtrace.ContextManager,
	noticeTable mrsql.DBTableInfo,
	queueTable mrsql.DBTableInfo,
	templateTableName string,
	templateVarName string,
	opts ...Option,
) *processconsume.MessageProcessor {
	o := options{
		defaultLang: defaultDefaultLang,
		processorOpts: []processconsume.Option{
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

	storageNotice := repository.NewNotePostgres(client, noticeTable)

	storageTemplate := templaterepository.NewTemplatePostgres(
		client,
		templateTableName,
	)

	storageTemplateVars := templaterepository.NewVariablePostgres(
		client,
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
	storageQueueCrashed := queuerepository.NewCrashedPostgres(
		client,
		mrsql.DBTableInfo{
			Name:       queueTable.Name + "_errors",
			PrimaryKey: queueTable.PrimaryKey,
		},
	)

	noticeConsumer := consume.New(
		client,
		storageNotice,
		queueconsume.New(
			client,
			storageQueue,
			queueconsume.WithStorageCompleted(storageQueueCompleted),
			queueconsume.WithStorageCrashed(storageQueueCrashed),
		),
	)

	return processconsume.NewMessageProcessor(
		noticeConsumer,
		handler.NewSendNotice(
			usecase.New(
				templateservice.New(
					storageTemplate,
					storageTemplateVars,
					logger,
					o.defaultLang,
				),
			),
			noticeProvider,
		),
		errorHandler,
		logger,
		traceManager,
		o.processorOpts...,
	)
}
