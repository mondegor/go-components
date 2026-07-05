package dbset

import (
	"github.com/mondegor/go-core/mrpostgres/builder/part"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/mrstorage/mrsql"

	"github.com/mondegor/go-components/mrsettings"
	"github.com/mondegor/go-components/mrsettings/field/format"
	"github.com/mondegor/go-components/mrsettings/repository"
	"github.com/mondegor/go-components/mrsettings/service"
)

// InitServiceSettingsSetter - создаёт объект для сохранения произвольных настроек в БД.
func InitServiceSettingsSetter(
	client mrstorage.DBConnManager,
	storageTable mrsql.DBTableInfo,
	storageTableLog string,
	opts ...Option,
) mrsettings.Setter {
	o := options{}

	for _, opt := range opts {
		opt(&o)
	}

	return service.NewSettingsSetter(
		client,
		format.New(o.formatterOpts...),
		repository.NewSettingPostgres(
			client,
			storageTable,
			part.NewSQLConditionBuilder(),
			o.storageCondition,
		),
		repository.NewSettingLogPostgres(
			client,
			storageTableLog,
			storageTable,
		),
	)
}
