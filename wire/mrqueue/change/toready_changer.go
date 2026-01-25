package change

import (
	"time"

	"github.com/mondegor/go-sysmess/mrevent"
	"github.com/mondegor/go-webcore/mrworker/helper"

	"github.com/mondegor/go-components/mrqueue/usecase/change/toready"
)

const (
	durationLimit = 15 * time.Second
)

// InitRetryToReadyChanger - создаёт объект RetryToReadyChanger.
func InitRetryToReadyChanger(
	storage toready.ItemStorage,
	eventEmitter mrevent.Emitter,
	opts ...toready.Option,
) *helper.ItemBatchPlayer {
	return helper.NewItemBatchPlayerWithDurationLimit(
		toready.New(
			storage,
			opts...,
		),
		mrevent.EmitterWithSource(eventEmitter, "StatusToReadyChanger"),
		durationLimit,
	)
}
