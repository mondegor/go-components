package clean

import (
	"time"

	"github.com/mondegor/go-core/mrevent"
	"github.com/mondegor/go-core/mrprocess/helper"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrauth/usecase/clean"
)

const (
	// durationLimit - предел длительности одного зацикленного воркера очистки (ItemBatchPlayer).
	durationLimit = 30 * time.Second
)

// InitAuthTokenCleaner - создаёт зацикленный воркер очистки истёкших auth-токенов.
func InitAuthTokenCleaner(
	txManager mrstorage.DBTxManager,
	storage clean.AuthTokenStorage,
	queue clean.SessionCleanupQueue,
	eventEmitter mrevent.Emitter,
) *helper.ItemBatchPlayer {
	return helper.NewItemBatchPlayerWithDurationLimit(
		clean.NewAuthTokenCleaner(txManager, storage, queue),
		mrevent.EmitterWithSource(eventEmitter, "AuthTokensCleaner"),
		durationLimit,
	)
}
