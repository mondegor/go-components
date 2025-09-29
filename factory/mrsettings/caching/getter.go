package caching

import (
	"context"
	"time"

	"github.com/mondegor/go-storage/mrpostgres/builder/part"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrlog"
	"github.com/mondegor/go-webcore/mrworker"
	"github.com/mondegor/go-webcore/mrworker/job/task"
	"github.com/mondegor/go-webcore/mrworker/process/schedule"

	core "github.com/mondegor/go-components/internal"
	"github.com/mondegor/go-components/mrsettings/bag/fieldparser"
	"github.com/mondegor/go-components/mrsettings/repository"
	"github.com/mondegor/go-components/mrsettings/usecase/cacheget"
)

const (
	defaultCaptionPrefix         = "Settings"
	defaultReloadSettingsCaption = "Task/CacheReloader"
	defaultReloadSettingsPeriod  = 5 * time.Minute
	defaultReloadSettingsTimeout = 15 * time.Second
)

type (
	getterOptions struct {
		captionPrefix      string
		fieldParser        []fieldparser.Option
		taskReloadSettings []task.Option
		storageCondition   mrstorage.SQLPartFunc
	}
)

// NewComponentGetter - создаёт получателя произвольных настроек из БД
// с использованием кэша и с периодическим его обновлением.
func NewComponentGetter(
	client mrstorage.DBConnManager,
	storageTable mrsql.DBTableInfo,
	errorHandler core.ErrorHandler,
	logger mrlog.Logger,
	traceManager core.TraceManager,
	opts ...GetterOption,
) (*cacheget.SettingsGetter, *schedule.TaskScheduler) {
	o := getterOptions{
		captionPrefix: defaultCaptionPrefix,
		taskReloadSettings: []task.Option{
			task.WithCaptionPrefix(defaultCaptionPrefix),
			task.WithCaption(defaultReloadSettingsCaption),
			task.WithPeriod(defaultReloadSettingsPeriod),
			task.WithTimeout(defaultReloadSettingsTimeout),
		},
	}

	for _, opt := range opts {
		opt(&o)
	}

	storage := repository.NewSettingPostgres(
		client,
		storageTable,
		part.NewSQLConditionBuilder(),
		o.storageCondition,
	)

	settingsReloader := cacheget.NewSettingsReloader(
		fieldparser.New(o.fieldParser...),
		storage,
		logger,
	)

	settingsReloaderTask := task.NewJobWrapper(
		mrworker.JobFunc(func(ctx context.Context) error {
			return settingsReloader.Reload(ctx)
		}),
		o.taskReloadSettings...,
	)

	return cacheget.NewSettingsGetter(settingsReloader),
		schedule.NewTaskScheduler(
			errorHandler,
			logger,
			traceManager,
			schedule.WithCaptionPrefix(o.captionPrefix),
			schedule.WithTasks(settingsReloaderTask),
		)
}
