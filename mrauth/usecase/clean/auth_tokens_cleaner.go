package clean

import (
	"context"

	"github.com/mondegor/go-sysmess/errors"
	"github.com/mondegor/go-sysmess/mrstorage"

	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// AuthTokenCleaner - объект, удаляющий истёкшие токены авторизации и ставящий
	// осиротевшие после этого сессии в очередь на удаление.
	AuthTokenCleaner struct {
		txManager    mrstorage.DBTxManager
		storage      authTokenStorage
		queue        sessionCleanupQueue
		errorWrapper errors.Wrapper
	}

	authTokenStorage interface {
		DeleteExpiredNonRefresh(ctx context.Context, limit int) (count int, err error)
		DeleteExpiredRefresh(ctx context.Context, limit int) (candidates []entity.SessionPK, err error)
	}

	sessionCleanupQueue interface {
		Enqueue(ctx context.Context, pks []entity.SessionPK) error
	}
)

// NewAuthTokenCleaner - создаёт объект AuthTokenCleaner.
func NewAuthTokenCleaner(
	txManager mrstorage.DBTxManager,
	storage authTokenStorage,
	queue sessionCleanupQueue,
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

	return nonRefreshCount + refreshCount, nil
}
