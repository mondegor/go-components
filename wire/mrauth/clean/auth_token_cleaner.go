package clean

import (
	"time"

	"github.com/mondegor/go-sysmess/mrevent"
	"github.com/mondegor/go-sysmess/mrprocess/helper"
	"github.com/mondegor/go-sysmess/mrstorage"

	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/usecase/clean"
)

const (
	// durationLimit - предел длительности одного зацикленного воркера очистки (ItemBatchPlayer).
	durationLimit = 30 * time.Second
)

// InitAuthTokenCleaner - создаёт зацикленный воркер очистки истёкших auth-токенов.
func InitAuthTokenCleaner(
	txManager mrstorage.DBTxManager,
	storage *repository.AuthTokenPostgres,
	queue *repository.SessionCleanupQueuePostgres,
	eventEmitter mrevent.Emitter,
) *helper.ItemBatchPlayer {
	return helper.NewItemBatchPlayerWithDurationLimit(
		clean.NewAuthTokenCleaner(txManager, storage, queue),
		mrevent.EmitterWithSource(eventEmitter, "AuthTokensCleaner"),
		durationLimit,
	)
}
