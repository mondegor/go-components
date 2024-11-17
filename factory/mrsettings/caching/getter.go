package caching

import (
	"time"

	"github.com/mondegor/go-storage/mrpostgres/builder/part"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-webcore/mrcore/mrapp"
	"github.com/mondegor/go-webcore/mrworker"
	"github.com/mondegor/go-webcore/mrworker/job/task"

	"github.com/mondegor/go-components/mrsettings/component/cacheget"
	"github.com/mondegor/go-components/mrsettings/features/fieldparser"
	"github.com/mondegor/go-components/mrsettings/repository"
)

const (
	defaultReloadSettingsCaption = "Settings.Reload"
	defaultReloadSettingsPeriod  = 5 * time.Minute
	defaultReloadSettingsTimeout = 15 * time.Second
)

type (
	getterOptions struct {
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
	opts ...GetterOption,
) (*cacheget.SettingsGetter, mrworker.Task) {
	options := getterOptions{
		taskReloadSettings: []task.Option{
			task.WithCaption(defaultReloadSettingsCaption),
			task.WithPeriod(defaultReloadSettingsPeriod),
			task.WithTimeout(defaultReloadSettingsTimeout),
		},
	}

	for _, opt := range opts {
		opt(&options)
	}

	sharedCache := cacheget.NewSharedCache()
	storage := repository.NewSettingPostgres(
		client,
		storageTable,
		part.NewSQLConditionBuilder(),
		mrapp.NewStorageErrorWrapper(),
		options.storageCondition,
	)

	getter := cacheget.NewSettingsGetter(
		sharedCache,
		storage,
	)

	reloadJobTask := task.NewJobWrapper(
		cacheget.NewReloadSettingsJob(
			sharedCache,
			fieldparser.New(options.fieldParser...),
			storage,
			mrapp.NewUseCaseErrorWrapper(),
		),
		options.taskReloadSettings...,
	)

	return getter, reloadJobTask
}
