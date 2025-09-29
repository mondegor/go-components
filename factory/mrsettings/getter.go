package mrsettings

import (
	"github.com/mondegor/go-storage/mrpostgres/builder/part"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"

	"github.com/mondegor/go-components/mrsettings/bag/fieldparser"
	"github.com/mondegor/go-components/mrsettings/repository"
	"github.com/mondegor/go-components/mrsettings/usecase/get"
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
	o := getterOptions{}

	for _, opt := range opts {
		opt(&o)
	}

	return get.New(
		fieldparser.New(o.fieldParser...),
		repository.NewSettingPostgres(
			client,
			storageTable,
			part.NewSQLConditionBuilder(),
			o.storageCondition,
		),
	)
}
