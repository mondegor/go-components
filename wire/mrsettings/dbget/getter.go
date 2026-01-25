package dbget

import (
	"github.com/mondegor/go-storage/mrpostgres/builder/part"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"

	"github.com/mondegor/go-components/mrsettings"
	"github.com/mondegor/go-components/mrsettings/field/parse"
	"github.com/mondegor/go-components/mrsettings/repository"
	"github.com/mondegor/go-components/mrsettings/service"
)

// InitServiceSettingsGetter - создаёт получателя произвольных настроек из БД.
func InitServiceSettingsGetter(
	client mrstorage.DBConnManager,
	storageTable mrsql.DBTableInfo,
	opts ...Option,
) mrsettings.Getter {
	o := options{}

	for _, opt := range opts {
		opt(&o)
	}

	return service.NewSettingsGetter(
		parse.New(o.parserOpts...),
		repository.NewSettingPostgres(
			client,
			storageTable,
			part.NewSQLConditionBuilder(),
			o.storageCondition,
		),
	)
}
