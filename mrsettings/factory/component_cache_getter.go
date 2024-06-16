package factory

import (
	"github.com/mondegor/go-storage/mrpostgres"
	"github.com/mondegor/go-storage/mrsql"

	"github.com/mondegor/go-components/mrsettings/cachegetter"
	"github.com/mondegor/go-components/mrsettings/fieldparser"
	"github.com/mondegor/go-components/mrsettings/lightgetter"
	"github.com/mondegor/go-components/mrsettings/repository"
)

type (
	// ComponentCacheGetterOptions - опции для создания ComponentCacheGetter.
	ComponentCacheGetterOptions ComponentGetterOptions
)

// NewComponentCacheGetter - создаёт объект cachegetter.Component.
func NewComponentCacheGetter(opts ComponentCacheGetterOptions) *cachegetter.Component {
	if opts.DBMeta == nil {
		opts.DBMeta = mrsql.NewEntityMeta("sample_catalog.settings", "setting_name", nil)
	}

	if opts.DBCondition == nil {
		opts.DBCondition = mrpostgres.NewSQLBuilderCondition(mrpostgres.NewSQLBuilderWhere())
	}

	return cachegetter.New(
		fieldparser.New(opts.ListItemSeparator),
		repository.New(
			opts.DBClient,
			opts.DBMeta,
			opts.DBCondition,
		),
		opts.ErrorWrapper,
	)
}

// NewComponentLightCacheGetter - создаёт объект lightgetter.Component.
func NewComponentLightCacheGetter(opts ComponentCacheGetterOptions) *lightgetter.Component {
	return lightgetter.New(
		NewComponentCacheGetter(opts),
	)
}
