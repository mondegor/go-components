package clean

import (
	"github.com/mondegor/go-sysmess/mrevent"
	"github.com/mondegor/go-sysmess/mrprocess/helper"
	"github.com/mondegor/go-sysmess/mrstorage"

	"github.com/mondegor/go-components/mrauth/usecase/clean"
)

// InitSessionExcessTrimmer - создаёт зацикленный воркер фоновой чистки лишних сессий: по каждому
// пользователю из очереди ревокает дубли устройства и старейшие сессии сверх лимита, затем сам
// удаляет осиротевшие строки.
func InitSessionExcessTrimmer(
	txManager mrstorage.DBTxManager,
	consumer clean.SessionExcessQueueConsumer,
	openFetcher clean.OpenSessionFetcher,
	lister clean.SessionLister,
	closer clean.SessionCloser,
	deleter clean.OrphanSessionDeleter,
	eventEmitter mrevent.Emitter,
) *helper.ItemBatchPlayer {
	return helper.NewItemBatchPlayerWithDurationLimit(
		clean.NewSessionExcessTrimmer(
			txManager,
			consumer,
			openFetcher,
			lister,
			closer,
			deleter,
		),
		mrevent.EmitterWithSource(eventEmitter, "SessionExcessTrimmer"),
		durationLimit,
	)
}
