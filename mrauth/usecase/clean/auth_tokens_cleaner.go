package clean

import (
	"context"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"

	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// AuthTokenCleaner - объект, удаляющий истёкшие токены авторизации и ставящий
	// осиротевшие после этого сессии в очередь на удаление.
	AuthTokenCleaner struct {
		txManager    mrstorage.DBTxManager
		storage      AuthTokenStorage
		queue        SessionCleanupQueue
		errorWrapper errors.Wrapper
	}

	// AuthTokenStorage - хранилище токенов авторизации для удаления истёкших.
	AuthTokenStorage interface {
		DeleteExpiredNonRefresh(ctx context.Context, limit int) (count int, err error)
		DeleteExpiredRefresh(ctx context.Context, limit int) (candidates []entity.SessionPK, err error)
	}

	// SessionCleanupQueue - очередь постановки осиротевших сессий на удаление.
	SessionCleanupQueue interface {
		Enqueue(ctx context.Context, pks []entity.SessionPK) error
	}
)

// NewAuthTokenCleaner - создаёт объект AuthTokenCleaner.
func NewAuthTokenCleaner(
	txManager mrstorage.DBTxManager,
	storage AuthTokenStorage,
	queue SessionCleanupQueue,
) *AuthTokenCleaner {
	return &AuthTokenCleaner{
		txManager:    txManager,
		storage:      storage,
		queue:        queue,
		errorWrapper: errors.NewServiceOperationFailedWrapper(),
	}
}

// Execute - удаляет одну пачку истёкших токенов (до limit не-refresh + до limit refresh)
// и ставит сессии удалённых refresh токенов в очередь на удаление (кандидатов). Возвращает
// суммарное число удалённых токенов - для ItemBatchPlayer это сигнал "пачка была полной, есть ещё".
// Сначала чистятся не-refresh токены, затем в одной транзакции удаляются
// refresh токены и их сессии ставятся в очередь: при сбое Enqueue удаление refresh токенов
// откатывается, и кандидаты будут найдены повторно на следующем проходе. Реальная осиротелость
// проверяется уже на стадии слива очереди.
//
// ВНИМАНИЕ: count - это сумма двух источников (не-refresh + refresh), каждый ограничен limit,
// поэтому за один вызов может быть удалено до 2*limit строк. Для ItemBatchPlayer это не приводит
// к раннему выходу из цикла (count >= max(источников), цикл продолжается пока есть полная пачка
// хотя бы у одного источника), но размывает контракт "limit = размер батча" и ~2x завышает
// итоговый total, эмитируемый ItemBatchPlayer.
func (co *AuthTokenCleaner) Execute(ctx context.Context, limit int) (count int, err error) {
	if limit < 1 {
		return 0, errors.ErrInternalIncorrectInputData.WithDetails("limit is zero or negative")
	}

	nonRefreshCount, err := co.storage.DeleteExpiredNonRefresh(ctx, limit)
	if err != nil {
		return 0, co.errorWrapper.Wrap(err)
	}

	refreshCount := 0

	err = co.txManager.Do(ctx, func(ctx context.Context) error {
		candidates, err := co.storage.DeleteExpiredRefresh(ctx, limit)
		if err != nil {
			return err
		}

		refreshCount = len(candidates)

		return co.queue.Enqueue(ctx, candidates)
	})
	if err != nil {
		return 0, co.errorWrapper.Wrap(err)
	}

	// TODO: count = сумма двух источников (до 2*limit) размывает контракт
	// "limit = размер батча" и ~2x завышает total у ItemBatchPlayer; гонять
	// источники (non-refresh / refresh) как отдельные ItemBatchPlayer-воркеры.
	return nonRefreshCount + refreshCount, nil
}
