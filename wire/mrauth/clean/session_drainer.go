package clean

import (
	"github.com/mondegor/go-sysmess/mrevent"
	"github.com/mondegor/go-sysmess/mrprocess/helper"

	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/mrauth/usecase/clean"
)

// InitSessionDrainer - создаёт зацикленный воркер слива очереди удаления сессий:
// consumer (очередь) выбирает пачку, deleter атомарно удаляет из неё реально осиротевшие строки сессий.
func InitSessionDrainer(
	consumer *repository.SessionCleanupQueuePostgres,
	deleter *repository.OrphanSessionDeleterPostgres,
	eventEmitter mrevent.Emitter,
) *helper.ItemBatchPlayer {
	return helper.NewItemBatchPlayerWithDurationLimit(
		clean.NewSessionDrainer(consumer, deleter),
		mrevent.EmitterWithSource(eventEmitter, "SessionCleanupDrainer"),
		durationLimit,
	)
}
