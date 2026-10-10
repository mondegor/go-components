package toretry_test

import (
	"context"
	"testing"
	"time"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrqueue/entity"
	"github.com/mondegor/go-components/mrqueue/usecase/change/toretry"
	"github.com/mondegor/go-components/mrqueue/usecase/change/toretry/mock"
)

//go:generate mockgen -source=status_changer.go -destination=mock/status_changer.go -package=mock
//go:generate mockgen -destination=mock/mrstorage.go -package=mock github.com/mondegor/go-core/mrstorage DBTxManager

// runJob - выполняет переданное в txManager.Do замыкание синхронно (без реальной транзакции).
func runJob(ctx context.Context, job func(context.Context) error, _ ...mrstorage.TxOption) error {
	return job(ctx)
}

func TestProcessingToRetryChanger_Execute(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	tx := mock.NewMockDBTxManager(ctrl)
	storage := mock.NewMockItemStorage(ctrl)

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	storage.EXPECT().UpdateStatusProcessingToRetryByTimeout(gomock.Any(), gomock.Any(), 10).DoAndReturn(
		func(_ context.Context, timeout time.Duration, _ int) ([]uint64, error) {
			// без опции используется таймаут по умолчанию
			assert.Positive(t, timeout)

			return []uint64{1, 2}, nil
		},
	)

	uc := toretry.New(tx, storage)

	count, err := uc.Execute(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestProcessingToRetryChanger_Execute_WithStorageCrashed(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	tx := mock.NewMockDBTxManager(ctrl)
	storage := mock.NewMockItemStorage(ctrl)
	storageCrashed := mock.NewMockcrashedItemStorage(ctrl)

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	storage.EXPECT().UpdateStatusProcessingToRetryByTimeout(gomock.Any(), 3*time.Second, 10).Return([]uint64{5, 7}, nil)
	storageCrashed.EXPECT().Insert(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, rows []entity.CrashedItem) error {
			// в журнал ошибок попадает каждый переведённый элемент с причиной перевода
			require.Len(t, rows, 2)
			assert.Equal(t, uint64(5), rows[0].ID)
			assert.Equal(t, uint64(7), rows[1].ID)
			assert.NotEmpty(t, rows[0].Cause)
			assert.Equal(t, rows[0].Cause, rows[1].Cause)

			return nil
		},
	)

	uc := toretry.New(
		tx,
		storage,
		toretry.WithRetryTimeout(3*time.Second),
		toretry.WithStorageCrashed(storageCrashed),
	)

	count, err := uc.Execute(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestProcessingToRetryChanger_Execute_NothingChanged(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	tx := mock.NewMockDBTxManager(ctrl)
	storage := mock.NewMockItemStorage(ctrl)
	storageCrashed := mock.NewMockcrashedItemStorage(ctrl)

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	storage.EXPECT().UpdateStatusProcessingToRetryByTimeout(gomock.Any(), gomock.Any(), 10).Return(nil, nil)
	// журнал ошибок не должен вызываться

	uc := toretry.New(tx, storage, toretry.WithStorageCrashed(storageCrashed))

	count, err := uc.Execute(context.Background(), 10)
	require.NoError(t, err)
	assert.Zero(t, count)
}

func TestProcessingToRetryChanger_Execute_IncorrectLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		limit int
	}{
		{name: "zero", limit: 0},
		{name: "negative", limit: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			// ни транзакция, ни хранилище не должны вызываться
			uc := toretry.New(mock.NewMockDBTxManager(ctrl), mock.NewMockItemStorage(ctrl))

			count, err := uc.Execute(context.Background(), tt.limit)
			require.ErrorIs(t, err, errors.ErrInternalIncorrectInputData)
			assert.Zero(t, count)
		})
	}
}

func TestProcessingToRetryChanger_Execute_StorageError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	tx := mock.NewMockDBTxManager(ctrl)
	storage := mock.NewMockItemStorage(ctrl)
	storageCrashed := mock.NewMockcrashedItemStorage(ctrl)

	errStorage := errors.New("storage failed")

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	storage.EXPECT().UpdateStatusProcessingToRetryByTimeout(gomock.Any(), gomock.Any(), 10).Return(nil, errStorage)
	// журнал ошибок не должен вызываться

	uc := toretry.New(tx, storage, toretry.WithStorageCrashed(storageCrashed))

	count, err := uc.Execute(context.Background(), 10)
	require.ErrorIs(t, err, errStorage)
	assert.Zero(t, count)
}

func TestProcessingToRetryChanger_Execute_StorageCrashedError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	tx := mock.NewMockDBTxManager(ctrl)
	storage := mock.NewMockItemStorage(ctrl)
	storageCrashed := mock.NewMockcrashedItemStorage(ctrl)

	errInsert := errors.New("insert failed")

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	storage.EXPECT().UpdateStatusProcessingToRetryByTimeout(gomock.Any(), gomock.Any(), 10).Return([]uint64{1}, nil)
	storageCrashed.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(errInsert)

	uc := toretry.New(tx, storage, toretry.WithStorageCrashed(storageCrashed))

	count, err := uc.Execute(context.Background(), 10)
	require.ErrorIs(t, err, errInsert)
	assert.Zero(t, count)
}
