package mrsettings

import (
	"github.com/mondegor/go-storage/mrpostgres/builder/part"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrevent"

	"github.com/mondegor/go-components/mrsettings/bag/fieldformatter"
	"github.com/mondegor/go-components/mrsettings/repository"
	"github.com/mondegor/go-components/mrsettings/usecase/set"
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
	eventEmitter mrevent.Emitter,
	useCaseErrorWrapper mrerr.UseCaseErrorWrapper,
	storageErrorWrapper mrerr.ErrorWrapper,
	storageTable mrsql.DBTableInfo,
	storageTableLog string,
	opts ...SetterOption,
) *set.SettingsSetter {
	o := setterOptions{}

	for _, opt := range opts {
		opt(&o)
	}

	return set.New(
		client,
		fieldformatter.New(o.fieldFormatter...),
		repository.NewSettingPostgres(
			client,
			storageErrorWrapper,
			storageTable,
			part.NewSQLConditionBuilder(),
			o.storageCondition,
		),
		repository.NewSettingLogPostgres(
			client,
			storageErrorWrapper,
			storageTableLog,
			storageTable,
		),
		eventEmitter,
		useCaseErrorWrapper,
	)
}
