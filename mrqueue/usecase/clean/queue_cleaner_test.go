package clean_test

import (
	"context"
	"testing"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrqueue/usecase/clean"
	"github.com/mondegor/go-components/mrqueue/usecase/clean/mock"
)

//go:generate mockgen -source=queue_cleaner.go -destination=mock/queue_cleaner.go -package=mock
//go:generate mockgen -destination=mock/mrstorage.go -package=mock github.com/mondegor/go-core/mrstorage DBTxManager

// runJob - выполняет переданное в txManager.Do замыкание синхронно (без реальной транзакции).
func runJob(ctx context.Context, job func(context.Context) error, _ ...mrstorage.TxOption) error {
	return job(ctx)
}

func TestQueueCleaner_Execute(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	tx := mock.NewMockDBTxManager(ctrl)
	storage := mock.NewMockItemStorage(ctrl)

	var cleanedIDs []uint64

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	storage.EXPECT().DeleteRetryWithoutAttempts(gomock.Any(), 10).Return([]uint64{1, 2, 3}, nil)

	uc := clean.New(
		tx,
		storage,
		clean.WithAfterClean(func(_ context.Context, itemsIDs []uint64) error {
			cleanedIDs = itemsIDs

			return nil
		}),
	)

	count, err := uc.Execute(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
	assert.Equal(t, []uint64{1, 2, 3}, cleanedIDs)
}

func TestQueueCleaner_Execute_WithoutAfterClean(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts []clean.Option
	}{
		{name: "option not set"},
		{name: "nil option", opts: []clean.Option{clean.WithAfterClean(nil)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			tx := mock.NewMockDBTxManager(ctrl)
			storage := mock.NewMockItemStorage(ctrl)

			tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
			storage.EXPECT().DeleteRetryWithoutAttempts(gomock.Any(), 10).Return([]uint64{1}, nil)

			uc := clean.New(tx, storage, tt.opts...)

			count, err := uc.Execute(context.Background(), 10)
			require.NoError(t, err)
			assert.Equal(t, 1, count)
		})
	}
}

func TestQueueCleaner_Execute_NothingCleaned(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	tx := mock.NewMockDBTxManager(ctrl)
	storage := mock.NewMockItemStorage(ctrl)

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	storage.EXPECT().DeleteRetryWithoutAttempts(gomock.Any(), 10).Return(nil, nil)

	uc := clean.New(
		tx,
		storage,
		clean.WithAfterClean(func(_ context.Context, _ []uint64) error {
			assert.Fail(t, "afterClean must not be called when nothing is cleaned")

			return nil
		}),
	)

	count, err := uc.Execute(context.Background(), 10)
	require.NoError(t, err)
	assert.Zero(t, count)
}

func TestQueueCleaner_Execute_IncorrectLimit(t *testing.T) {
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
			uc := clean.New(mock.NewMockDBTxManager(ctrl), mock.NewMockItemStorage(ctrl))

			count, err := uc.Execute(context.Background(), tt.limit)
			require.ErrorIs(t, err, errors.ErrInternalIncorrectInputData)
			assert.Zero(t, count)
		})
	}
}

func TestQueueCleaner_Execute_StorageError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	tx := mock.NewMockDBTxManager(ctrl)
	storage := mock.NewMockItemStorage(ctrl)

	errStorage := errors.New("storage failed")

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	storage.EXPECT().DeleteRetryWithoutAttempts(gomock.Any(), 10).Return(nil, errStorage)

	uc := clean.New(tx, storage)

	count, err := uc.Execute(context.Background(), 10)
	require.ErrorIs(t, err, errStorage)
	assert.Zero(t, count)
}

func TestQueueCleaner_Execute_AfterCleanError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	tx := mock.NewMockDBTxManager(ctrl)
	storage := mock.NewMockItemStorage(ctrl)

	errAfterClean := errors.New("after clean failed")

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob)
	storage.EXPECT().DeleteRetryWithoutAttempts(gomock.Any(), 10).Return([]uint64{1}, nil)

	uc := clean.New(
		tx,
		storage,
		clean.WithAfterClean(func(_ context.Context, _ []uint64) error {
			return errAfterClean
		}),
	)

	count, err := uc.Execute(context.Background(), 10)
	require.ErrorIs(t, err, errAfterClean)
	assert.Zero(t, count)
}
