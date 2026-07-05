package clean

import (
	"time"

	"github.com/mondegor/go-core/mrevent"
	"github.com/mondegor/go-core/mrprocess/helper"

	"github.com/mondegor/go-components/mrauth/usecase/clean"
)

// InitOperationCleaner - создаёт зацикленный воркер очистки просроченных секретных операций
// и старых записей их лога.
func InitOperationCleaner(
	storage clean.OperationStorage,
	storageLog clean.OperationLogStorage,
	logLifeTime time.Duration,
	eventEmitter mrevent.Emitter,
) *helper.ItemBatchPlayer {
	return helper.NewItemBatchPlayerWithDurationLimit(
		clean.NewOperationCleaner(storage, storageLog, logLifeTime),
		mrevent.EmitterWithSource(eventEmitter, "OperationCleaner"),
		durationLimit,
	)
}
