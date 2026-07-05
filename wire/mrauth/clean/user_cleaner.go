package clean

import (
	"time"

	"github.com/mondegor/go-core/mrevent"
	"github.com/mondegor/go-core/mrprocess/helper"

	"github.com/mondegor/go-components/mrauth/usecase/clean"
)

// InitUserCleaner - создаёт зацикленный воркер очистки старых записей лога активности пользователей.
func InitUserCleaner(
	storageLog clean.UserActivityLogStorage,
	logLifeTime time.Duration,
	eventEmitter mrevent.Emitter,
) *helper.ItemBatchPlayer {
	return helper.NewItemBatchPlayerWithDurationLimit(
		clean.NewUserCleaner(storageLog, logLifeTime),
		mrevent.EmitterWithSource(eventEmitter, "UserActivityCleaner"),
		durationLimit,
	)
}
