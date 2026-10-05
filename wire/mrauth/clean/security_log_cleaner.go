package clean

import (
	"time"

	"github.com/mondegor/go-core/mrevent"
	"github.com/mondegor/go-core/mrprocess/helper"

	"github.com/mondegor/go-components/mrauth/usecase/clean"
)

// InitSecurityLogCleaner - создаёт зацикленный воркер очистки старых записей журнала безопасности пользователей.
func InitSecurityLogCleaner(
	storageLog clean.SecurityLogStorage,
	logLifeTime time.Duration,
	eventEmitter mrevent.Emitter,
) *helper.ItemBatchPlayer {
	return helper.NewItemBatchPlayerWithDurationLimit(
		clean.NewSecurityLogCleaner(storageLog, logLifeTime),
		mrevent.EmitterWithSource(eventEmitter, "SecurityLogCleaner"),
		durationLimit,
	)
}
