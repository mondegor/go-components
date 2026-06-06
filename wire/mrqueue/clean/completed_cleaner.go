package clean

import (
	"github.com/mondegor/go-sysmess/mrevent"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/mondegor/go-sysmess/mrworker/helper"

	"github.com/mondegor/go-components/mrqueue/usecase/completed/clean"
)

// InitCompletedItemsCleaner - создаёт объект CompletedItemsCleaner.
func InitCompletedItemsCleaner(
	txManager mrstorage.DBTxManager,
	storage clean.ItemStorage,
	eventEmitter mrevent.Emitter,
	opts ...clean.Option,
) *helper.ItemBatchPlayer {
	return helper.NewItemBatchPlayerWithDurationLimit(
		clean.New(
			txManager,
			storage,
			opts...,
		),
		mrevent.EmitterWithSource(eventEmitter, "CompletedItemsCleaner"),
		durationLimit,
	)
}
