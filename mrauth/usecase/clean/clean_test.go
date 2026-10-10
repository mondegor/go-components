package clean_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	sysmesserrors "github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/usecase/clean"
	"github.com/mondegor/go-components/mrauth/usecase/clean/mock"
)

//go:generate mockgen -source=auth_tokens_cleaner.go -destination=mock/auth_tokens_cleaner.go -package=mock
//go:generate mockgen -source=operation_cleaner.go -destination=mock/operation_cleaner.go -package=mock
//go:generate mockgen -source=operation_log_cleaner.go -destination=mock/operation_log_cleaner.go -package=mock
//go:generate mockgen -source=security_log_cleaner.go -destination=mock/security_log_cleaner.go -package=mock
//go:generate mockgen -source=user_cleaner.go -destination=mock/user_cleaner.go -package=mock
//go:generate mockgen -source=session_drainer.go -destination=mock/session_drainer.go -package=mock
//go:generate mockgen -destination=mock/mrstorage.go -package=mock github.com/mondegor/go-core/mrstorage DBTxManager

// runJob - выполняет переданный в txManager.Do замыкание синхронно (без реальной транзакции).
func runJob(_ context.Context, job func(context.Context) error, _ ...mrstorage.TxOption) error {
	return job(context.Background())
}

// ----- AuthTokenCleaner -----

func TestAuthTokenCleaner_Execute_SumsCounts(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	tx := mock.NewMockDBTxManager(ctrl)
	storage := mock.NewMockAuthTokenStorage(ctrl)
	queue := mock.NewMockSessionCleanupQueue(ctrl)

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
	storage := mock.NewMockAuthTokenStorage(ctrl)
	queue := mock.NewMockSessionCleanupQueue(ctrl)

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
	storage := mock.NewMockAuthTokenStorage(ctrl)
	queue := mock.NewMockSessionCleanupQueue(ctrl)

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

func TestOperationCleaner_Execute(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	storage := mock.NewMockOperationStorage(ctrl)
	storage.EXPECT().DeleteExpired(gomock.Any(), 100).Return(3, nil)

	uc := clean.NewOperationCleaner(storage)

	count, err := uc.Execute(context.Background(), 100)
	require.NoError(t, err)
	require.Equal(t, 3, count)
}

// ----- OperationLogCleaner -----

func TestOperationLogCleaner_Execute(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	before := time.Now().UTC().Add(-time.Hour)

	storageLog := mock.NewMockOperationLogStorage(ctrl)
	storageLog.EXPECT().DeleteBeforeDate(gomock.Any(), gomock.Any(), 100).DoAndReturn(
		func(_ context.Context, datetime time.Time, _ int) (int, error) {
			// граница удаления отсчитывается от текущего момента на срок хранения назад
			assert.WithinDuration(t, before, datetime, time.Minute)

			return 4, nil
		},
	)

	uc := clean.NewOperationLogCleaner(storageLog, time.Hour)

	count, err := uc.Execute(context.Background(), 100)
	require.NoError(t, err)
	require.Equal(t, 4, count)
}

// ----- SecurityLogCleaner -----

func TestSecurityLogCleaner_Execute(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	before := time.Now().UTC().Add(-time.Hour)

	storageLog := mock.NewMockSecurityLogStorage(ctrl)
	storageLog.EXPECT().DeleteBeforeDate(gomock.Any(), gomock.Any(), 100).DoAndReturn(
		func(_ context.Context, datetime time.Time, _ int) (int, error) {
			// граница удаления отсчитывается от текущего момента на срок хранения назад
			assert.WithinDuration(t, before, datetime, time.Minute)

			return 5, nil
		},
	)

	uc := clean.NewSecurityLogCleaner(storageLog, time.Hour)

	count, err := uc.Execute(context.Background(), 100)
	require.NoError(t, err)
	require.Equal(t, 5, count)
}

// ----- UserCleaner -----

func TestUserCleaner_Execute(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	storageLog := mock.NewMockUserActivityLogStorage(ctrl)
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

	consumer := mock.NewMockSessionCleanupQueueConsumer(ctrl)
	deleter := mock.NewMockOrphanSessionDeleter(ctrl)

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

	consumer := mock.NewMockSessionCleanupQueueConsumer(ctrl)
	deleter := mock.NewMockOrphanSessionDeleter(ctrl)

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

	consumer := mock.NewMockSessionCleanupQueueConsumer(ctrl)
	deleter := mock.NewMockOrphanSessionDeleter(ctrl)

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

	consumer := mock.NewMockSessionCleanupQueueConsumer(ctrl)
	deleter := mock.NewMockOrphanSessionDeleter(ctrl)

	consumer.EXPECT().Fetch(gomock.Any(), 100).Return(nil, errors.New("fetch failed"))

	uc := clean.NewSessionDrainer(consumer, deleter)

	_, err := uc.Execute(context.Background(), 100)
	require.Error(t, err)
}

// ----- общие для очистителей проверки -----

// TestCleaners_Execute_InvalidLimit - нулевой или отрицательный размер пачки - ошибка проводки:
// хранилища не вызываются (моки без EXPECT: любой вызов провалит тест).
func TestCleaners_Execute_InvalidLimit(t *testing.T) {
	t.Parallel()

	type executor interface {
		Execute(ctx context.Context, limit int) (int, error)
	}

	tests := []struct {
		name  string
		newUC func(ctrl *gomock.Controller) executor
	}{
		{
			name: "auth token cleaner",
			newUC: func(ctrl *gomock.Controller) executor {
				return clean.NewAuthTokenCleaner(
					mock.NewMockDBTxManager(ctrl), mock.NewMockAuthTokenStorage(ctrl), mock.NewMockSessionCleanupQueue(ctrl),
				)
			},
		},
		{
			name: "operation cleaner",
			newUC: func(ctrl *gomock.Controller) executor {
				return clean.NewOperationCleaner(mock.NewMockOperationStorage(ctrl))
			},
		},
		{
			name: "operation log cleaner",
			newUC: func(ctrl *gomock.Controller) executor {
				return clean.NewOperationLogCleaner(mock.NewMockOperationLogStorage(ctrl), time.Hour)
			},
		},
		{
			name: "security log cleaner",
			newUC: func(ctrl *gomock.Controller) executor {
				return clean.NewSecurityLogCleaner(mock.NewMockSecurityLogStorage(ctrl), time.Hour)
			},
		},
		{
			name: "user cleaner",
			newUC: func(ctrl *gomock.Controller) executor {
				return clean.NewUserCleaner(mock.NewMockUserActivityLogStorage(ctrl), time.Hour)
			},
		},
		{
			name: "session drainer",
			newUC: func(ctrl *gomock.Controller) executor {
				return clean.NewSessionDrainer(mock.NewMockSessionCleanupQueueConsumer(ctrl), mock.NewMockOrphanSessionDeleter(ctrl))
			},
		},
		{
			name: "session excess trimmer",
			newUC: func(ctrl *gomock.Controller) executor {
				return newExcessTrimmerMocks(ctrl).uc
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := tt.newUC(gomock.NewController(t))

			for _, limit := range []int{0, -1} {
				count, err := uc.Execute(context.Background(), limit)
				require.ErrorIs(t, err, sysmesserrors.ErrInternalIncorrectInputData)
				assert.Zero(t, count)
			}
		})
	}
}

// TestLogCleaners_Execute_StorageError - сбой хранилища возвращается ошибкой, count = 0.
func TestLogCleaners_Execute_StorageError(t *testing.T) {
	t.Parallel()

	errStorage := errors.New("storage failed")

	type executor interface {
		Execute(ctx context.Context, limit int) (int, error)
	}

	tests := []struct {
		name  string
		newUC func(ctrl *gomock.Controller) executor
	}{
		{
			name: "operation cleaner",
			newUC: func(ctrl *gomock.Controller) executor {
				storage := mock.NewMockOperationStorage(ctrl)
				storage.EXPECT().DeleteExpired(gomock.Any(), 100).Return(0, errStorage)

				return clean.NewOperationCleaner(storage)
			},
		},
		{
			name: "operation log cleaner",
			newUC: func(ctrl *gomock.Controller) executor {
				storage := mock.NewMockOperationLogStorage(ctrl)
				storage.EXPECT().DeleteBeforeDate(gomock.Any(), gomock.Any(), 100).Return(0, errStorage)

				return clean.NewOperationLogCleaner(storage, time.Hour)
			},
		},
		{
			name: "security log cleaner",
			newUC: func(ctrl *gomock.Controller) executor {
				storage := mock.NewMockSecurityLogStorage(ctrl)
				storage.EXPECT().DeleteBeforeDate(gomock.Any(), gomock.Any(), 100).Return(0, errStorage)

				return clean.NewSecurityLogCleaner(storage, time.Hour)
			},
		},
		{
			name: "user cleaner",
			newUC: func(ctrl *gomock.Controller) executor {
				storage := mock.NewMockUserActivityLogStorage(ctrl)
				storage.EXPECT().DeleteBeforeDate(gomock.Any(), gomock.Any(), 100).Return(0, errStorage)

				return clean.NewUserCleaner(storage, time.Hour)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			count, err := tt.newUC(gomock.NewController(t)).Execute(context.Background(), 100)
			require.ErrorIs(t, err, errStorage)
			assert.Zero(t, count)
		})
	}
}

// TestAuthTokenCleaner_Execute_RefreshErrorSkipsEnqueue - сбой удаления refresh токенов
// откатывает транзакцию: кандидаты в очередь не ставятся.
func TestAuthTokenCleaner_Execute_RefreshErrorSkipsEnqueue(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	tx := mock.NewMockDBTxManager(ctrl)
	storage := mock.NewMockAuthTokenStorage(ctrl)
	queue := mock.NewMockSessionCleanupQueue(ctrl)

	errRefresh := errors.New("refresh failed")

	storage.EXPECT().DeleteExpiredNonRefresh(gomock.Any(), 100).Return(5, nil)
	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	storage.EXPECT().DeleteExpiredRefresh(gomock.Any(), 100).Return(nil, errRefresh)
	// queue.Enqueue не должен вызываться

	uc := clean.NewAuthTokenCleaner(tx, storage, queue)

	count, err := uc.Execute(context.Background(), 100)
	require.ErrorIs(t, err, errRefresh)
	assert.Zero(t, count)
}

// TestSessionDrainer_Execute_AckError - сбой ack возвращается ошибкой: пачка будет
// переобработана на следующем проходе (удаление идемпотентно).
func TestSessionDrainer_Execute_AckError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	consumer := mock.NewMockSessionCleanupQueueConsumer(ctrl)
	deleter := mock.NewMockOrphanSessionDeleter(ctrl)

	pks := []entity.SessionPK{{UserID: uuid.New(), SessionID: 1}}
	errAck := errors.New("ack failed")

	gomock.InOrder(
		consumer.EXPECT().Fetch(gomock.Any(), 100).Return(pks, nil),
		deleter.EXPECT().DeleteOrphaned(gomock.Any(), pks).Return(nil),
		consumer.EXPECT().Delete(gomock.Any(), pks).Return(errAck),
	)

	uc := clean.NewSessionDrainer(consumer, deleter)

	count, err := uc.Execute(context.Background(), 100)
	require.ErrorIs(t, err, errAck)
	assert.Zero(t, count)
}
