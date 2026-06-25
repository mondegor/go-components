package clean_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-sysmess/mrstorage"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/usecase/clean"
	"github.com/mondegor/go-components/mrauth/usecase/clean/mock"
)

//go:generate mockgen -source=auth_tokens_cleaner.go -destination=mock/auth_tokens_cleaner.go -package=mock
//go:generate mockgen -source=operation_cleaner.go -destination=mock/operation_cleaner.go -package=mock
//go:generate mockgen -source=user_cleaner.go -destination=mock/user_cleaner.go -package=mock
//go:generate mockgen -source=session_drainer.go -destination=mock/session_drainer.go -package=mock
//go:generate mockgen -destination=mock/mrstorage.go -package=mock github.com/mondegor/go-sysmess/mrstorage DBTxManager

// runJob - выполняет переданный в txManager.Do замыкание синхронно (без реальной транзакции).
func runJob(_ context.Context, job func(context.Context) error, _ ...mrstorage.TxOption) error {
	return job(context.Background())
}

// ----- AuthTokenCleaner -----

func TestAuthTokenCleaner_Execute_SumsCounts(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	tx := mock.NewMockDBTxManager(ctrl)
	storage := mock.NewMockauthTokenStorage(ctrl)
	queue := mock.NewMocksessionCleanupQueue(ctrl)

	candidates := []entity.SessionPK{
		{UserID: uuid.New(), SessionID: 1},
		{UserID: uuid.New(), SessionID: 2},
	}

	storage.EXPECT().DeleteExpiredNonRefresh(gomock.Any(), 100).Return(5, nil)
	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	storage.EXPECT().DeleteExpiredRefresh(gomock.Any(), 100).Return(candidates, nil)
	queue.EXPECT().Enqueue(gomock.Any(), candidates).Return(nil)

	uc := clean.NewAuthTokenCleaner(tx, storage, queue)

	count, err := uc.Execute(context.Background(), 100)
	require.NoError(t, err)
	require.Equal(t, 7, count) // 5 не-refresh + 2 refresh
}

func TestAuthTokenCleaner_Execute_NonRefreshErrorSkipsTx(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	tx := mock.NewMockDBTxManager(ctrl)
	storage := mock.NewMockauthTokenStorage(ctrl)
	queue := mock.NewMocksessionCleanupQueue(ctrl)

	storage.EXPECT().DeleteExpiredNonRefresh(gomock.Any(), 100).Return(0, errors.New("boom"))
	// tx.Do / DeleteExpiredRefresh / Enqueue не должны вызываться

	uc := clean.NewAuthTokenCleaner(tx, storage, queue)

	_, err := uc.Execute(context.Background(), 100)
	require.Error(t, err)
}

func TestAuthTokenCleaner_Execute_EnqueueErrorPropagates(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	tx := mock.NewMockDBTxManager(ctrl)
	storage := mock.NewMockauthTokenStorage(ctrl)
	queue := mock.NewMocksessionCleanupQueue(ctrl)

	candidates := []entity.SessionPK{{UserID: uuid.New(), SessionID: 1}}

	storage.EXPECT().DeleteExpiredNonRefresh(gomock.Any(), 100).Return(0, nil)
	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	storage.EXPECT().DeleteExpiredRefresh(gomock.Any(), 100).Return(candidates, nil)
	queue.EXPECT().Enqueue(gomock.Any(), candidates).Return(errors.New("enqueue failed"))

	uc := clean.NewAuthTokenCleaner(tx, storage, queue)

	_, err := uc.Execute(context.Background(), 100)
	require.Error(t, err)
}

// ----- OperationCleaner -----

func TestOperationCleaner_Execute_SumsCounts(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	storage := mock.NewMockoperationStorage(ctrl)
	storageLog := mock.NewMockoperationLogStorage(ctrl)

	storage.EXPECT().DeleteExpired(gomock.Any(), 100).Return(3, nil)
	storageLog.EXPECT().DeleteBeforeDate(gomock.Any(), gomock.Any(), 100).Return(4, nil)

	uc := clean.NewOperationCleaner(storage, storageLog, time.Hour)

	count, err := uc.Execute(context.Background(), 100)
	require.NoError(t, err)
	require.Equal(t, 7, count)
}

// ----- UserCleaner -----

func TestUserCleaner_Execute(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	storageLog := mock.NewMockuserActivityLogStorage(ctrl)
	storageLog.EXPECT().DeleteBeforeDate(gomock.Any(), gomock.Any(), 100).Return(9, nil)

	uc := clean.NewUserCleaner(storageLog, time.Hour)

	count, err := uc.Execute(context.Background(), 100)
	require.NoError(t, err)
	require.Equal(t, 9, count)
}

// ----- SessionDrainer -----

func TestSessionDrainer_Execute_EmptyQueue(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	consumer := mock.NewMocksessionCleanupQueueConsumer(ctrl)
	deleter := mock.NewMockorphanSessionDeleter(ctrl)

	consumer.EXPECT().Fetch(gomock.Any(), 100).Return([]entity.SessionPK{}, nil)
	// DeleteOrphaned / consumer.Delete не вызываются при пустой пачке

	uc := clean.NewSessionDrainer(consumer, deleter)

	count, err := uc.Execute(context.Background(), 100)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestSessionDrainer_Execute_HappyPath(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	consumer := mock.NewMocksessionCleanupQueueConsumer(ctrl)
	deleter := mock.NewMockorphanSessionDeleter(ctrl)

	pks := []entity.SessionPK{
		{UserID: uuid.New(), SessionID: 1},
		{UserID: uuid.New(), SessionID: 2},
	}

	gomock.InOrder(
		consumer.EXPECT().Fetch(gomock.Any(), 100).Return(pks, nil),
		deleter.EXPECT().DeleteOrphaned(gomock.Any(), pks).Return(nil),
		consumer.EXPECT().Delete(gomock.Any(), pks).Return(nil), // ack после удаления
	)

	uc := clean.NewSessionDrainer(consumer, deleter)

	count, err := uc.Execute(context.Background(), 100)
	require.NoError(t, err)
	require.Equal(t, len(pks), count) // возвращается размер пачки, не число удалённых
}

func TestSessionDrainer_Execute_DeleteErrorSkipsAck(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	consumer := mock.NewMocksessionCleanupQueueConsumer(ctrl)
	deleter := mock.NewMockorphanSessionDeleter(ctrl)

	pks := []entity.SessionPK{{UserID: uuid.New(), SessionID: 1}}

	consumer.EXPECT().Fetch(gomock.Any(), 100).Return(pks, nil)
	deleter.EXPECT().DeleteOrphaned(gomock.Any(), pks).Return(errors.New("delete failed"))
	// consumer.Delete (ack) НЕ должен вызываться - иначе at-least-once нарушится

	uc := clean.NewSessionDrainer(consumer, deleter)

	_, err := uc.Execute(context.Background(), 100)
	require.Error(t, err)
}

func TestSessionDrainer_Execute_FetchError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	consumer := mock.NewMocksessionCleanupQueueConsumer(ctrl)
	deleter := mock.NewMockorphanSessionDeleter(ctrl)

	consumer.EXPECT().Fetch(gomock.Any(), 100).Return(nil, errors.New("fetch failed"))

	uc := clean.NewSessionDrainer(consumer, deleter)

	_, err := uc.Execute(context.Background(), 100)
	require.Error(t, err)
}
