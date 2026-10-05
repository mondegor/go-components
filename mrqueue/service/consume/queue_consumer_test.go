package consume_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mondegor/go-core/mrstorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrqueue/entity"
	"github.com/mondegor/go-components/mrqueue/service/consume"
	"github.com/mondegor/go-components/mrqueue/service/consume/mock"
)

//go:generate mockgen -source=queue_consumer.go -destination=mock/queue_consumer.go -package=mock
//go:generate mockgen -destination=mock/mrstorage.go -package=mock github.com/mondegor/go-core/mrstorage DBTxManager

// TestQueueConsumerRejectStorableCause - текст ошибки попадает в журнал ошибок без невалидного
// UTF-8 и NUL (иначе вставка откатила бы транзакцию отклонения) и не длиннее предела,
// обрезанный по границе символа; переводы строк сохраняются.
func TestQueueConsumerRejectStorableCause(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name  string
		cause string
		want  string
	}

	tests := []testCase{
		{name: "plain multiline", cause: "550 rejected\nsee details", want: "550 rejected\nsee details"},
		{name: "invalid utf8 and nul", cause: "a\xffb\x00c", want: "abc"},
		{name: "too long", cause: strings.Repeat("ж", 5000), want: strings.Repeat("ж", 4096)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			txManager := mock.NewMockDBTxManager(ctrl)
			storage := mock.NewMockitemStorage(ctrl)
			storageCrashed := mock.NewMockcrashedItemStorage(ctrl)

			txManager.EXPECT().
				Do(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, job func(ctx context.Context) error, _ ...mrstorage.TxOption) error {
					return job(ctx)
				})

			storage.EXPECT().Delete(gomock.Any(), uint64(1), gomock.Any()).Return(nil)

			var got entity.CrashedItem

			storageCrashed.EXPECT().
				InsertOne(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, row entity.CrashedItem) error {
					got = row

					return nil
				})

			consumer := consume.NewQueueConsumer(txManager, storage, consume.WithStorageCrashed(storageCrashed))

			require.NoError(t, consumer.Reject(context.Background(), 1, errors.New(tt.cause)))
			assert.True(t, utf8.ValidString(got.Cause))
			assert.Equal(t, tt.want, got.Cause)
		})
	}
}
