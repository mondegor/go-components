package scheduler

import (
	"context"
	"time"

	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/errorwrapper"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrtrace"
	"github.com/mondegor/go-webcore/mrworker"
	"github.com/mondegor/go-webcore/mrworker/job/task"
	"github.com/mondegor/go-webcore/mrworker/process/schedule"

	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/usecase/clean"
)

const (
	defaultCaptionPrefix = "Auth"
	defaultCleanLimit    = 100
	defaultLogLifeTime   = 7 * 24 * time.Hour

	defaultCleanRecordsCaption = "CleanRecords"
	defaultCleanRecordsPeriod  = 45 * time.Minute
	defaultCleanRecordsTimeout = 120 * time.Second
)

type (
	serviceOptions struct {
		captionPrefix    string
		cleanLimit       int
		logLifeTime      time.Duration
		taskCleanRecords []task.Option
	}
)

// NewService - создаёт сервис для обработки и отправки сообщений и связанных с ним задачи.
func NewService(
	client mrstorage.DBConnManager,
	errorHandler mrerr.ErrorHandler,
	logger mrlog.Logger,
	traceManager mrtrace.ContextManager,
	authTokenTable mrsql.DBTableInfo,
	operationTable mrsql.DBTableInfo,
	operationLogTable string,
	userActivityLogTable string,
	opts ...ServiceOption,
) *schedule.TaskScheduler {
	o := serviceOptions{
		captionPrefix: defaultCaptionPrefix,
		cleanLimit:    defaultCleanLimit,
		logLifeTime:   defaultLogLifeTime,
		taskCleanRecords: []task.Option{
			task.WithCaptionPrefix(defaultCaptionPrefix),
			task.WithCaption(defaultCleanRecordsCaption),
			task.WithPeriod(defaultCleanRecordsPeriod),
			task.WithTimeout(defaultCleanRecordsTimeout),
		},
	}

	for _, opt := range opts {
		opt(&o)
	}

	authTokenCleaner := clean.NewAuthTokenCleaner(
		repository.NewAuthTokenPostgres(
			client,
			errorwrapper.NewInfraStorage(),
			authTokenTable,
		),
		errorwrapper.NewUseCase(),
	)

	operationCleaner := clean.NewOperationCleaner(
		repository.NewSecureOperationPostgres(
			client,
			errorwrapper.NewInfraStorage(),
			operationTable,
		),
		repository.NewSecureOperationLogPostgres(
			client,
			errorwrapper.NewInfraStorage(),
			operationLogTable,
		),
		errorwrapper.NewUseCase(),
	)

	userCleaner := clean.NewUserCleaner(
		repository.NewUserActivityLogPostgres(
			client,
			errorwrapper.NewInfraStorage(),
			userActivityLogTable,
		),
		errorwrapper.NewUseCase(),
	)

	cleanerTask := task.NewJobWrapper(
		mrworker.JobFunc(func(ctx context.Context) error {
			if err := authTokenCleaner.RemoveExpired(ctx, o.cleanLimit); err != nil {
				return err
			}

			if err := operationCleaner.RemoveExpired(ctx, o.cleanLimit); err != nil {
				return err
			}

			if err := operationCleaner.RemoveOldLog(ctx, o.logLifeTime, o.cleanLimit); err != nil {
				return err
			}

			return userCleaner.RemoveOldLog(ctx, o.logLifeTime, o.cleanLimit)
		}),
		o.taskCleanRecords...,
	)

	return schedule.NewTaskScheduler(
		errorHandler,
		logger,
		traceManager,
		schedule.WithCaptionPrefix(o.captionPrefix),
		schedule.WithTasks(cleanerTask),
	)
}
