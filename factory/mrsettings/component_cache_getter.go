package mrsettings

import (
	"github.com/mondegor/go-storage/mrpostgres"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-webcore/mrcore"

	"github.com/mondegor/go-components/mrsettings/component/cachegetter"
	"github.com/mondegor/go-components/mrsettings/features/fieldparser"
	"github.com/mondegor/go-components/mrsettings/repository"
)

type (
	// ComponentCacheGetterOptions - опции для создания ComponentCacheGetter.
	ComponentCacheGetterOptions ComponentGetterOptions
)

// NewComponentCacheGetter - создаёт объект cachegetter.Component.
func NewComponentCacheGetter(
	client mrstorage.DBConnManager,
	meta mrstorage.MetaGetter,
	errorWrapper mrcore.UseCaseErrorWrapper,
	opts ComponentCacheGetterOptions,
) *cachegetter.Component {
	if opts.DBCondition == nil {
		opts.DBCondition = mrpostgres.NewSQLBuilderCondition(mrpostgres.NewSQLBuilderWhere())
	}

	return cachegetter.New(
		fieldparser.New(opts.ListItemSeparator),
		repository.New(client, meta, opts.DBCondition),
		errorWrapper,
	)
}
