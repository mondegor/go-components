package change

import (
	"github.com/mondegor/go-core/mrevent"
	"github.com/mondegor/go-core/mrprocess/helper"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrqueue/usecase/change/toretry"
)

// InitProcessingToRetryChanger - создаёт объект ProcessingToRetryChanger.
func InitProcessingToRetryChanger(
	txManager mrstorage.DBTxManager,
	storage toretry.ItemStorage,
	eventEmitter mrevent.Emitter,
	opts ...toretry.Option,
) *helper.ItemBatchPlayer {
	return helper.NewItemBatchPlayerWithDurationLimit(
		toretry.New(
			txManager,
			storage,
			opts...,
		),
		mrevent.EmitterWithSource(eventEmitter, "StatusToRetryChanger"),
		durationLimit,
	)
}
