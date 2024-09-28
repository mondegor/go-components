package mrsettings

import (
	"github.com/mondegor/go-storage/mrpostgres"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-webcore/mrcore"

	"github.com/mondegor/go-components/mrsettings/component/getter"
	"github.com/mondegor/go-components/mrsettings/features/fieldparser"
	"github.com/mondegor/go-components/mrsettings/repository"
)

type (
	// ComponentGetterOptions - опции для создания ComponentGetter.
	ComponentGetterOptions struct {
		ListItemSeparator string
		DBCondition       mrstorage.SQLBuilderCondition
	}
)

// NewComponentGetter - создаёт объект getter.Component.
func NewComponentGetter(
	client mrstorage.DBConnManager,
	meta mrstorage.MetaGetter,
	errorWrapper mrcore.UseCaseErrorWrapper,
	opts ComponentGetterOptions,
) *getter.Component {
	if opts.DBCondition == nil {
		opts.DBCondition = mrpostgres.NewSQLBuilderCondition(mrpostgres.NewSQLBuilderWhere())
	}

	return getter.New(
		fieldparser.New(opts.ListItemSeparator),
		repository.New(client, meta, opts.DBCondition),
		errorWrapper,
	)
}
