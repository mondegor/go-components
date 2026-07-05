package clean

import (
	"time"

	"github.com/mondegor/go-core/mrevent"
	"github.com/mondegor/go-core/mrprocess/helper"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrqueue/usecase/clean"
)

const (
	durationLimit = 30 * time.Second
)

// InitForgottenItemsCleaner - создаёт объект ForgottenItemsCleaner.
func InitForgottenItemsCleaner(
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
		mrevent.EmitterWithSource(eventEmitter, "ForgottenItemsCleaner"),
		durationLimit,
	)
}
