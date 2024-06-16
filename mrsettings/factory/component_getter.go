package factory

import (
	"github.com/mondegor/go-storage/mrpostgres"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-webcore/mrcore"

	"github.com/mondegor/go-components/mrsettings/fieldparser"
	"github.com/mondegor/go-components/mrsettings/getter"
	"github.com/mondegor/go-components/mrsettings/lightgetter"
	"github.com/mondegor/go-components/mrsettings/repository"
)

type (
	// ComponentGetterOptions - опции для создания ComponentGetter.
	ComponentGetterOptions struct {
		ListItemSeparator string // optional
		DBClient          mrstorage.DBConnManager
		DBMeta            mrstorage.MetaGetter          // optional
		DBCondition       mrstorage.SQLBuilderCondition // optional
		ErrorWrapper      mrcore.UsecaseErrorWrapper
	}
)

// NewComponentGetter - создаёт объект getter.Component.
func NewComponentGetter(opts ComponentGetterOptions) *getter.Component {
	if opts.DBMeta == nil {
		opts.DBMeta = mrsql.NewEntityMeta("sample_catalog.settings", "setting_id", nil)
	}

	if opts.DBCondition == nil {
		opts.DBCondition = mrpostgres.NewSQLBuilderCondition(mrpostgres.NewSQLBuilderWhere())
	}

	return getter.New(
		fieldparser.New(opts.ListItemSeparator),
		repository.New(
			opts.DBClient,
			opts.DBMeta,
			opts.DBCondition,
		),
		opts.ErrorWrapper,
	)
}

// NewComponentLightGetter - создаёт объект lightgetter.Component.
func NewComponentLightGetter(opts ComponentGetterOptions) *lightgetter.Component {
	return lightgetter.New(
		NewComponentGetter(opts),
	)
}
