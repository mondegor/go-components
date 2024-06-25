package mrsort

import (
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrsender"

	"github.com/mondegor/go-components/mrsort/component/orderer"

	"github.com/mondegor/go-components/mrsort/repository"
)

type (
	// ComponentOptions - опции для создания Component.
	ComponentOptions struct {
		DBClient     mrstorage.DBConnManager
		EventEmitter mrsender.EventEmitter
		ErrorWrapper mrcore.UsecaseErrorWrapper
	}
)

// NewComponentOrderer - создаёт объект orderer.Component.
func NewComponentOrderer(opts ComponentOptions) *orderer.Component {
	storage := repository.NewRepository(
		opts.DBClient,
	)

	return orderer.New(
		storage,
		opts.EventEmitter,
		opts.ErrorWrapper,
	)
}
