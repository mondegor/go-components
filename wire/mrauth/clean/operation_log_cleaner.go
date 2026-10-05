package clean

import (
	"time"

	"github.com/mondegor/go-core/mrevent"
	"github.com/mondegor/go-core/mrprocess/helper"

	"github.com/mondegor/go-components/mrauth/usecase/clean"
)

// InitOperationLogCleaner - создаёт зацикленный воркер очистки старых записей журнала защищённых операций.
func InitOperationLogCleaner(
	storageLog clean.OperationLogStorage,
	logLifeTime time.Duration,
	eventEmitter mrevent.Emitter,
) *helper.ItemBatchPlayer {
	return helper.NewItemBatchPlayerWithDurationLimit(
		clean.NewOperationLogCleaner(storageLog, logLifeTime),
		mrevent.EmitterWithSource(eventEmitter, "OperationLogCleaner"),
		durationLimit,
	)
}
