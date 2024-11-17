package mrsettings

import (
	"github.com/mondegor/go-storage/mrpostgres/builder/part"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-webcore/mrcore/mrapp"
	"github.com/mondegor/go-webcore/mrsender"

	"github.com/mondegor/go-components/mrsettings/component/set"
	"github.com/mondegor/go-components/mrsettings/features/fieldformatter"
	"github.com/mondegor/go-components/mrsettings/repository"
)

type (
	setterOptions struct {
		fieldFormatter   []fieldformatter.Option
		storageCondition mrstorage.SQLPartFunc
	}
)

// NewComponentSetter - создаёт объект для сохранения произвольных настроек в БД.
func NewComponentSetter(
	client mrstorage.DBConnManager,
	storageTable mrsql.DBTableInfo,
	eventEmitter mrsender.EventEmitter,
	opts ...SetterOption,
) *set.SettingsSetter {
	options := setterOptions{}

	for _, opt := range opts {
		opt(&options)
	}

	return set.New(
		fieldformatter.New(options.fieldFormatter...),
		repository.NewSettingPostgres(
			client,
			storageTable,
			part.NewSQLConditionBuilder(),
			mrapp.NewStorageErrorWrapper(),
			options.storageCondition,
		),
		eventEmitter,
		mrapp.NewUseCaseErrorWrapper(),
	)
}
