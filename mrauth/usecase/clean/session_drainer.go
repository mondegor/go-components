package clean

import (
	"context"

	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// SessionDrainer - воркер, сливающий очередь удаления сессий батчами.
	// Разделение ответственности: consumer выбирает/удаляет пары из очереди, deleter атомарно
	// удаляет из них реально осиротевшие строки сессий (проверка осиротелости и удаление - в
	// одном запросе, что исключает гонку с параллельным переоткрытием сессии).
	SessionDrainer struct {
		consumer     SessionCleanupQueueConsumer
		deleter      OrphanSessionDeleter
		errorWrapper errors.Wrapper
	}

	// SessionCleanupQueueConsumer - очередь удаления сессий: выборка и подтверждение пачки.
	SessionCleanupQueueConsumer interface {
		Fetch(ctx context.Context, limit int) ([]entity.SessionPK, error)
		Delete(ctx context.Context, pks []entity.SessionPK) error
	}

	// OrphanSessionDeleter - атомарное удаление реально осиротевших строк сессий.
	OrphanSessionDeleter interface {
		DeleteOrphaned(ctx context.Context, candidates []entity.SessionPK) error
	}
)

// NewSessionDrainer - создаёт объект SessionDrainer.
func NewSessionDrainer(
	consumer SessionCleanupQueueConsumer,
	deleter OrphanSessionDeleter,
) *SessionDrainer {
	return &SessionDrainer{
		consumer:     consumer,
		deleter:      deleter,
		errorWrapper: errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - обрабатывает одну пачку очереди (до limit пар) и возвращает её размер
// (для ItemBatchPlayer: count < limit = очередь иссякла). Возвращается именно размер пачки,
// а не число удалённых сессий - иначе при батче из неосиротевших кандидатов count был бы
// < limit и цикл оборвался бы раньше времени, оставив backlog в очереди.
// Надёжность (at-least-once): ack (consumer.Delete) делается ПОСЛЕ успешного удаления,
// поэтому краш между удалением и ack приводит лишь к идемпотентной переобработке, без потерь.
func (co *SessionDrainer) Execute(ctx context.Context, limit int) (count int, err error) {
	if limit < 1 {
		return 0, errors.ErrInternalIncorrectInputData.WithDetails("limit is zero or negative")
	}

	pks, err := co.consumer.Fetch(ctx, limit)
	if err != nil {
		return 0, co.errorWrapper.Wrap(err)
	}

	if len(pks) == 0 {
		return 0, nil
	}

	if err = co.deleter.DeleteOrphaned(ctx, pks); err != nil {
		return 0, co.errorWrapper.Wrap(err)
	}

	if err = co.consumer.Delete(ctx, pks); err != nil {
		return 0, co.errorWrapper.Wrap(err)
	}

	return len(pks), nil
}
