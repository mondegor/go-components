package scheduler

import (
	"context"
	"time"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrevent"
	"github.com/mondegor/go-core/mrlog"
	"github.com/mondegor/go-core/mrprocess"
	"github.com/mondegor/go-core/mrprocess/job/task"
	"github.com/mondegor/go-core/mrprocess/schedule"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/mrtrace"

	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/wire/mrauth/clean"
)

const (
	defaultCaptionPrefix = "Auth"
	defaultCleanLimit    = 100

	defaultOperationLogLifeTime = 30 * 24 * time.Hour
	defaultActivityLogLifeTime  = 7 * 24 * time.Hour
	defaultSecurityLogLifeTime  = 365 * 24 * time.Hour

	defaultCleanRecordsCaption = "CleanRecords"

	// defaultCleanRecordsPeriod - период между запусками очистки записей. Низкий дью-цикл:
	// воркеры на ItemBatchPlayer выходят сразу при первом неполном батче, поэтому в простое
	// короткий период почти бесплатен, а под churn'ом ограничивает рост и bloat очередей.
	// TODO: добавить алармы по глубине sessions_cleanup_queue и возрасту старейшего created_at,
	// чтобы ловить ситуацию, когда даже этого периода не хватает (до партиционирования).
	defaultCleanRecordsPeriod = 10 * time.Minute

	// defaultCleanRecordsTimeout - таймаут задачи должен перекрывать суммарную длительность
	// всех зацикленных воркеров очистки (каждый крутится до своего предела длительности,
	// см. wire/mrauth/clean), иначе под большим backlog'ом последний воркер упрётся в дедлайн контекста.
	defaultCleanRecordsTimeout = 300 * time.Second

	defaultTrimSessionsCaption = "TrimSessions"
	defaultTrimSessionsPeriod  = 5 * time.Minute
	defaultTrimSessionsTimeout = 60 * time.Second
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
	usersSecurityLogTableName,
	sessionsTableName,
	sessionsCleanupQueueTableName,
	sessionsExcessQueueTableName string,
	opts ...Option,
) *schedule.TaskScheduler {
	o := options{
		captionPrefix: defaultCaptionPrefix,
		taskCleanerOpts: []task.Option{
			task.WithCaptionPrefix(defaultCaptionPrefix),
			task.WithCaption(defaultCleanRecordsCaption),
			task.WithPeriod(defaultCleanRecordsPeriod),
			task.WithTimeout(defaultCleanRecordsTimeout),
		},
		taskTrimSessionsOpts: []task.Option{
			task.WithCaptionPrefix(defaultCaptionPrefix),
			task.WithCaption(defaultTrimSessionsCaption),
			task.WithPeriod(defaultTrimSessionsPeriod),
			task.WithTimeout(defaultTrimSessionsTimeout),
		},
	}

	for _, opt := range opts {
		opt(&o)
	}

	if o.cleanLimit < 1 {
		o.cleanLimit = defaultCleanLimit
	}

	if o.operationLogLifeTime < 1 {
		o.operationLogLifeTime = defaultOperationLogLifeTime
	}

	if o.activityLogLifeTime < 1 {
		o.activityLogLifeTime = defaultActivityLogLifeTime
	}

	if o.securityLogLifeTime < 1 {
		o.securityLogLifeTime = defaultSecurityLogLifeTime
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

	// клинер просроченных операций крутится в цикле до опустошения/таймаута
	operationCleaner := clean.InitOperationCleaner(
		repository.NewSecureOperationPostgres(
			client,
			secureOperationTableName,
		),
		eventEmitter,
	)

	// клинер журнала защищённых операций крутится в цикле до опустошения/таймаута
	operationLogCleaner := clean.InitOperationLogCleaner(
		repository.NewSecureOperationLogPostgres(
			client,
			secureOperationLogTableName,
		),
		o.operationLogLifeTime,
		eventEmitter,
	)

	// клинер лога активности пользователей крутится в цикле до опустошения/таймаута
	userCleaner := clean.InitUserCleaner(
		repository.NewUserActivityLogPostgres(
			client,
			usersActivityLogTableName,
		),
		o.activityLogLifeTime,
		eventEmitter,
	)

	// клинер журнала безопасности пользователей крутится в цикле до опустошения/таймаута
	securityLogCleaner := clean.InitSecurityLogCleaner(
		repository.NewUserSecurityLogPostgres(
			client,
			usersSecurityLogTableName,
		),
		o.securityLogLifeTime,
		eventEmitter,
	)

	orphanSessionDeleter := repository.NewOrphanSessionDeleterPostgres(client, sessionsTableName, authTokensTableName)

	// drain-воркер сливает очередь удаления сессий батчами до её опустошения/таймаута
	sessionDrainer := clean.InitSessionDrainer(
		sessionCleanupQueue,
		orphanSessionDeleter,
		eventEmitter,
	)

	// триммер лишних сессий: по каждому пользователю из очереди ревокает дубли устройства и
	// старейшие сессии сверх лимита, затем сам удаляет осиротевшие строки
	sessionExcessTrimmer := clean.InitSessionExcessTrimmer(
		client,
		repository.NewSessionExcessQueuePostgres(client, sessionsExcessQueueTableName),
		authTokenStorage, // openFetcher
		repository.NewSessionPostgres(client, sessionsTableName), // lister
		authTokenStorage, // closer
		orphanSessionDeleter,
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

			if err := operationLogCleaner.Execute(ctx, o.cleanLimit); err != nil {
				return err
			}

			if err := userCleaner.Execute(ctx, o.cleanLimit); err != nil {
				return err
			}

			if err := securityLogCleaner.Execute(ctx, o.cleanLimit); err != nil {
				return err
			}

			// слив очереди сессий - последним: токен-клинер выше наполнил её
			// осиротевшими сессиями (закрытыми/просроченными)
			return sessionDrainer.Execute(ctx, o.cleanLimit)
		}),
		o.taskCleanerOpts...,
	)

	trimSessionsTask := task.NewJobWrapper(
		mrprocess.JobFunc(func(ctx context.Context) error {
			return sessionExcessTrimmer.Execute(ctx, o.cleanLimit)
		}),
		o.taskTrimSessionsOpts...,
	)

	return schedule.NewTaskScheduler(
		errorHandler,
		logger,
		traceManager,
		schedule.WithCaptionPrefix(o.captionPrefix),
		schedule.WithTasks(cleanerTask, trimSessionsTask),
	)
}
