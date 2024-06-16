package factory

import (
	"github.com/mondegor/go-storage/mrpostgres"
	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrsender"

	"github.com/mondegor/go-components/mrsettings/fieldformatter"
	"github.com/mondegor/go-components/mrsettings/repository"
	"github.com/mondegor/go-components/mrsettings/setter"
)

type (
	// ComponentSetterOptions - опции для создания ComponentGetter.
	ComponentSetterOptions struct {
		ValueMaxLen       uint64 // optional
		ListItemSeparator string // optional
		DBClient          mrstorage.DBConnManager
		DBMeta            mrstorage.MetaGetter          // optional
		DBCondition       mrstorage.SQLBuilderCondition // optional
		EventEmitter      mrsender.EventEmitter
		ErrorWrapper      mrcore.UsecaseErrorWrapper
	}
)

// NewComponentSetter - создаёт объект setter.Component.
func NewComponentSetter(opts ComponentSetterOptions) *setter.Component {
	if opts.DBMeta == nil {
		opts.DBMeta = mrsql.NewEntityMeta("sample_catalog.settings", "setting_name", nil)
	}

	if opts.DBCondition == nil {
		opts.DBCondition = mrpostgres.NewSQLBuilderCondition(mrpostgres.NewSQLBuilderWhere())
	}

	return setter.New(
		fieldformatter.New(
			fieldformatter.DBFieldFormatterOptions{
				ValueMaxLen:       opts.ValueMaxLen,
				ListItemSeparator: opts.ListItemSeparator,
			},
		),
		repository.New(
			opts.DBClient,
			opts.DBMeta,
			opts.DBCondition,
		),
		opts.EventEmitter,
		opts.ErrorWrapper,
	)
}
