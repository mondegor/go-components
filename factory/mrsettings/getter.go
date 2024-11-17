package mrsettings

import (
	"github.com/mondegor/go-storage/mrpostgres/builder/part"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-webcore/mrcore/mrapp"

	"github.com/mondegor/go-components/mrsettings/component/get"
	"github.com/mondegor/go-components/mrsettings/features/fieldparser"
	"github.com/mondegor/go-components/mrsettings/repository"
)

type (
	getterOptions struct {
		fieldParser      []fieldparser.Option
		storageCondition mrstorage.SQLPartFunc
	}
)

// NewComponentGetter - создаёт получателя произвольных настроек из БД.
func NewComponentGetter(
	client mrstorage.DBConnManager,
	storageTable mrsql.DBTableInfo,
	opts ...GetterOption,
) *get.SettingsGetter {
	options := getterOptions{}

	for _, opt := range opts {
		opt(&options)
	}

	return get.New(
		fieldparser.New(options.fieldParser...),
		repository.NewSettingPostgres(
			client,
			storageTable,
			part.NewSQLConditionBuilder(),
			mrapp.NewStorageErrorWrapper(),
			options.storageCondition,
		),
		mrapp.NewUseCaseErrorWrapper(),
	)
}
