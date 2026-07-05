package dbget

import (
	"github.com/mondegor/go-sysmess/mrpostgres/builder/part"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-sysmess/mrstorage/mrsql"

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
