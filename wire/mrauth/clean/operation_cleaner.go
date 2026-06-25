package clean

import (
	"time"

	"github.com/mondegor/go-sysmess/mrevent"
	"github.com/mondegor/go-sysmess/mrprocess/helper"

	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/usecase/clean"
)

// InitOperationCleaner - создаёт зацикленный воркер очистки просроченных секретных операций
// и старых записей их лога.
func InitOperationCleaner(
	storage *repository.SecureOperationPostgres,
	storageLog *repository.SecureOperationLogPostgres,
	logLifeTime time.Duration,
	eventEmitter mrevent.Emitter,
) *helper.ItemBatchPlayer {
	return helper.NewItemBatchPlayerWithDurationLimit(
		clean.NewOperationCleaner(storage, storageLog, logLifeTime),
		mrevent.EmitterWithSource(eventEmitter, "OperationCleaner"),
		durationLimit,
	)
}
