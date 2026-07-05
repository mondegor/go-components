package cacheget

import (
	"context"
	"time"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrlog"
	"github.com/mondegor/go-core/mrpostgres/builder/part"
	"github.com/mondegor/go-core/mrprocess"
	"github.com/mondegor/go-core/mrprocess/job/task"
	"github.com/mondegor/go-core/mrprocess/schedule"
	"github.com/mondegor/go-core/mrrun"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/mrstorage/mrsql"
	"github.com/mondegor/go-core/mrtrace"

	"github.com/mondegor/go-components/mrsettings"
	"github.com/mondegor/go-components/mrsettings/field/parse"
	"github.com/mondegor/go-components/mrsettings/repository"
	"github.com/mondegor/go-components/mrsettings/service/cacheget"
)

const (
	defaultCaptionPrefix         = "Settings"
	defaultReloadSettingsCaption = "Task/CacheReloader"
	defaultReloadSettingsPeriod  = 5 * time.Minute
	defaultReloadSettingsTimeout = 15 * time.Second
)

// InitServiceSettingsGetter - создаёт получателя произвольных настроек из БД
// с использованием кэша и с периодическим его обновлением.
func InitServiceSettingsGetter(
	client mrstorage.DBConnManager,
	storageTable mrsql.DBTableInfo,
	errorHandler errors.Handler,
	logger mrlog.Logger,
	traceManager mrtrace.ContextManager,
	opts ...Option,
) (mrsettings.Getter, mrrun.Process) {
	o := options{
		captionPrefix: defaultCaptionPrefix,
		taskReloaderOpts: []task.Option{
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

	serviceSettingsGetter := cacheget.NewSettingsGetter(
		parse.New(o.parserOpts...),
		storage,
		logger,
	)

	return serviceSettingsGetter,
		schedule.NewTaskScheduler(
			errorHandler,
			logger,
			traceManager,
			schedule.WithCaptionPrefix(o.captionPrefix),
			schedule.WithTasks(
				task.NewJobWrapper(
					mrprocess.JobFunc(func(ctx context.Context) error {
						return serviceSettingsGetter.Reload(ctx)
					}),
					o.taskReloaderOpts...,
				),
			),
		)
}
