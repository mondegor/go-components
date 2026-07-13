package produce_test

import (
	"context"
	"testing"
	"time"

	"github.com/mondegor/go-core/mrlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/component/produce"
	"github.com/mondegor/go-components/mrauth/entity"
)

type (
	// fakeLogProducer - продюсер, который либо принимает запись, либо имитирует переполненную
	// очередь коллектора: PushMessage блокируется до истечения переданного контекста.
	fakeLogProducer struct {
		blocked bool
		entries []entity.SecureOperationLog
	}
)

func (f *fakeLogProducer) PushMessage(ctx context.Context, entry entity.SecureOperationLog) error {
	if f.blocked {
		<-ctx.Done()

		return ctx.Err()
	}

	f.entries = append(f.entries, entry)

	return nil
}

func TestSecureOperationLogger_Log_PushesToCollector(t *testing.T) {
	t.Parallel()

	producer := &fakeLogProducer{}
	logger := produce.NewSecureOperationLogger(producer, mrlog.NopLogger())

	entry := entity.SecureOperationLog{OperationName: "confirm.authorize.user"}
	logger.Log(context.Background(), entry)

	require.Len(t, producer.entries, 1)
	assert.Equal(t, "confirm.authorize.user", producer.entries[0].OperationName)
}

// TestSecureOperationLogger_Log_OverflowDoesNotBlockLong - журнал best-effort: при заторе в
// коллекторе вызов не должен надолго удерживать горутину запроса.
func TestSecureOperationLogger_Log_OverflowDoesNotBlockLong(t *testing.T) {
	t.Parallel()

	logger := produce.NewSecureOperationLogger(&fakeLogProducer{blocked: true}, mrlog.NopLogger())

	start := time.Now()

	logger.Log(context.Background(), entity.SecureOperationLog{OperationName: "confirm.authorize.user"})

	assert.Less(t, time.Since(start), time.Second, "ожидание места в очереди ограничено pushTimeout")
}

// TestSecureOperationLogger_Log_NilProducerDisabledNoPanic - хост может не поднимать коллектор
// журнала: вместо паники на nil-продюсере журнал молча отключается.
func TestSecureOperationLogger_Log_NilProducerDisabledNoPanic(t *testing.T) {
	t.Parallel()

	logger := produce.NewSecureOperationLogger(nil, mrlog.NopLogger())

	assert.NotPanics(t, func() {
		logger.Log(context.Background(), entity.SecureOperationLog{OperationName: "confirm.authorize.user"})
	})
}

// TestSecureOperationLogger_Log_CanceledRequestStillLogs - отмена запроса не должна отменять
// запись: контекст продюсера отвязан от контекста запроса.
func TestSecureOperationLogger_Log_CanceledRequestStillLogs(t *testing.T) {
	t.Parallel()

	producer := &fakeLogProducer{}
	logger := produce.NewSecureOperationLogger(producer, mrlog.NopLogger())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	logger.Log(ctx, entity.SecureOperationLog{OperationName: "confirm.authorize.user"})

	require.Len(t, producer.entries, 1)
}
