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
	"github.com/mondegor/go-sysmess/mrtrace"

	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/wire/mrauth/clean"
)

const (
	defaultCaptionPrefix = "Auth"
	defaultCleanLimit    = 100
	defaultLogLifeTime   = 7 * 24 * time.Hour

	defaultCleanRecordsCaption = "CleanRecords"
	defaultCleanRecordsPeriod  = 45 * time.Minute

	// defaultCleanRecordsTimeout - таймаут задачи должен перекрывать суммарную длительность
	// всех зацикленных воркеров очистки (каждый крутится до своего предела длительности,
	// см. wire/mrauth/clean), иначе под большим backlog'ом последний воркер упрётся в дедлайн контекста.
	defaultCleanRecordsTimeout = 300 * time.Second
)

// NewService - создаёт планировщик фоновых задач очистки модуля Auth.
//
// ВАЖНО: планировщик рассчитан на запуск в ЕДИНСТВЕННОМ экземпляре (single-pod либо под
// leader-election). Конвейер очистки не имеет конкурентной защиты на выборке: SessionCleanupQueue.Fetch
// и удаления просроченных операций / логов / активности выбирают строки без блокировки,
// поэтому при нескольких одновременно работающих экземплярах они дублировали бы работу и/или
// блокировали друг друга на одних и тех же строках.
func NewService(
	client mrstorage.DBConnManager,
	eventEmitter mrevent.Emitter,
	errorHandler errors.Handler,
	logger mrlog.Logger,
	traceManager mrtrace.ContextManager,
	authTokensTableName,
	secureOperationTableName,
	secureOperationLogTableName,
	usersActivityLogTableName,
	sessionsTableName,
	sessionsCleanupQueueTableName string,
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

	authTokenStorage := repository.NewAuthTokenPostgres(
		client,
		authTokensTableName,
	)

	sessionCleanupQueue := repository.NewSessionCleanupQueuePostgres(
		client,
		sessionsCleanupQueueTableName,
	)

	// клинер токенов крутится в цикле до опустошения/таймаута
	authTokensCleaner := clean.InitAuthTokenCleaner(
		client,
		authTokenStorage,
		sessionCleanupQueue,
		eventEmitter,
	)

	// клинер операций крутится в цикле до опустошения/таймаута:
	// бандлит удаление просроченных операций и старых записей их лога
	operationCleaner := clean.InitOperationCleaner(
		repository.NewSecureOperationPostgres(
			client,
			secureOperationTableName,
		),
		repository.NewSecureOperationLogPostgres(
			client,
			secureOperationLogTableName,
		),
		o.logLifeTime,
		eventEmitter,
	)

	// клинер лога активности пользователей крутится в цикле до опустошения/таймаута
	userCleaner := clean.InitUserCleaner(
		repository.NewUserActivityLogPostgres(
			client,
			usersActivityLogTableName,
		),
		o.logLifeTime,
		eventEmitter,
	)

	// drain-воркер сливает очередь удаления сессий батчами до её опустошения/таймаута
	sessionDrainer := clean.InitSessionDrainer(
		sessionCleanupQueue,
		repository.NewOrphanSessionDeleterPostgres(client, sessionsTableName, authTokensTableName),
		eventEmitter,
	)

	cleanerTask := task.NewJobWrapper(
		mrprocess.JobFunc(func(ctx context.Context) error {
			if err := authTokensCleaner.Execute(ctx, o.cleanLimit); err != nil {
				return err
			}

			if err := operationCleaner.Execute(ctx, o.cleanLimit); err != nil {
				return err
			}

			if err := userCleaner.Execute(ctx, o.cleanLimit); err != nil {
				return err
			}

			// слив очереди сессий - последним: токен-клинер выше наполнил её
			// осиротевшими сессиями (закрытыми/просроченными)
			return sessionDrainer.Execute(ctx, o.cleanLimit)
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
