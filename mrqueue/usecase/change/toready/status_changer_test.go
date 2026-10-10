package toready_test

import (
	"context"
	"testing"
	"time"

	"github.com/mondegor/go-core/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrqueue/usecase/change/toready"
	"github.com/mondegor/go-components/mrqueue/usecase/change/toready/mock"
)

//go:generate mockgen -source=status_changer.go -destination=mock/status_changer.go -package=mock

func TestRetryToReadyChanger_Execute(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	storage := mock.NewMockItemStorage(ctrl)
	storage.EXPECT().UpdateStatusRetryToReady(gomock.Any(), gomock.Any(), 10).DoAndReturn(
		func(_ context.Context, delayed time.Duration, _ int) ([]uint64, error) {
			// без опции используется задержка по умолчанию
			assert.Positive(t, delayed)

			return []uint64{1, 2, 3}, nil
		},
	)

	uc := toready.New(storage)

	count, err := uc.Execute(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestRetryToReadyChanger_Execute_WithRetryDelayed(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	storage := mock.NewMockItemStorage(ctrl)
	storage.EXPECT().UpdateStatusRetryToReady(gomock.Any(), 7*time.Second, 10).Return(nil, nil)

	uc := toready.New(storage, toready.WithRetryDelayed(7*time.Second))

	count, err := uc.Execute(context.Background(), 10)
	require.NoError(t, err)
	assert.Zero(t, count)
}

func TestRetryToReadyChanger_Execute_IncorrectLimit(t *testing.T) {
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

			// хранилище не должно вызываться
			uc := toready.New(mock.NewMockItemStorage(ctrl))

			count, err := uc.Execute(context.Background(), tt.limit)
			require.ErrorIs(t, err, errors.ErrInternalIncorrectInputData)
			assert.Zero(t, count)
		})
	}
}

func TestRetryToReadyChanger_Execute_StorageError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	errStorage := errors.New("storage failed")

	storage := mock.NewMockItemStorage(ctrl)
	storage.EXPECT().UpdateStatusRetryToReady(gomock.Any(), gomock.Any(), 10).Return(nil, errStorage)

	uc := toready.New(storage)

	count, err := uc.Execute(context.Background(), 10)
	require.ErrorIs(t, err, errStorage)
	assert.Zero(t, count)
}
