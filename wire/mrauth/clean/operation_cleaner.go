package clean

import (
	"github.com/mondegor/go-core/mrevent"
	"github.com/mondegor/go-core/mrprocess/helper"

	"github.com/mondegor/go-components/mrauth/usecase/clean"
)

// InitOperationCleaner - создаёт зацикленный воркер очистки просроченных защищённых операций.
func InitOperationCleaner(
	storage clean.OperationStorage,
	eventEmitter mrevent.Emitter,
) *helper.ItemBatchPlayer {
	return helper.NewItemBatchPlayerWithDurationLimit(
		clean.NewOperationCleaner(storage),
		mrevent.EmitterWithSource(eventEmitter, "OperationCleaner"),
		durationLimit,
	)
}
