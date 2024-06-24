package factory

import (
	"github.com/mondegor/go-storage/mrpostgres"
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
		ValueMaxLen       uint64
		ListItemSeparator string
		DBCondition       mrstorage.SQLBuilderCondition
	}
)

// NewComponentSetter - создаёт объект setter.Component.
func NewComponentSetter(
	client mrstorage.DBConnManager,
	meta mrstorage.MetaGetter,
	eventEmitter mrsender.EventEmitter,
	errorWrapper mrcore.UsecaseErrorWrapper,
	opts ComponentSetterOptions,
) *setter.Component {
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
		repository.New(client, meta, opts.DBCondition),
		eventEmitter,
		errorWrapper,
	)
}
