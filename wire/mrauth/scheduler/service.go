package scheduler

import (
	"context"
	"time"

	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-sysmess/mrprocess"
	"github.com/mondegor/go-sysmess/mrprocess/job/task"
	"github.com/mondegor/go-sysmess/mrprocess/schedule"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-sysmess/mrtrace"

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

// NewService - создаёт сервис для обработки и отправки сообщений и связанных с ним задачи.
func NewService(
	client mrstorage.DBConnManager,
	errorHandler errors.Handler,
	logger mrlog.Logger,
	traceManager mrtrace.ContextManager,
	authTokensTableName,
	secureOperationTableName,
	secureOperationLogTableName,
	usersActivityLogTableName string,
	opts ...Option,
) *schedule.TaskScheduler {
	o := options{
		captionPrefix: defaultCaptionPrefix,
		logLifeTime:   defaultLogLifeTime,
		taskCleanerOpts: []task.Option{
			task.WithCaptionPrefix(defaultCaptionPrefix),
			task.WithCaption(defaultCleanRecordsCaption),
			task.WithPeriod(defaultCleanRecordsPeriod),
			task.WithTimeout(defaultCleanRecordsTimeout),
		},
	}

	for _, opt := range opts {
		opt(&o)
	}

	if o.cleanLimit < 1 {
		o.cleanLimit = defaultCleanLimit
	}

	authTokenCleaner := clean.NewAuthTokenCleaner(
		repository.NewAuthTokenPostgres(
			client,
			authTokensTableName,
		),
	)

	operationCleaner := clean.NewOperationCleaner(
		repository.NewSecureOperationPostgres(
			client,
			secureOperationTableName,
		),
		repository.NewSecureOperationLogPostgres(
			client,
			secureOperationLogTableName,
		),
	)

	userCleaner := clean.NewUserCleaner(
		repository.NewUserActivityLogPostgres(
			client,
			usersActivityLogTableName,
		),
	)

	cleanerTask := task.NewJobWrapper(
		mrprocess.JobFunc(func(ctx context.Context) error {
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
		o.taskCleanerOpts...,
	)

	return schedule.NewTaskScheduler(
		errorHandler,
		logger,
		traceManager,
		schedule.WithCaptionPrefix(o.captionPrefix),
		schedule.WithTasks(cleanerTask),
	)
}
