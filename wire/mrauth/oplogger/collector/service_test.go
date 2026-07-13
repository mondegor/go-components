package collector_test

import (
	"context"
	"testing"

	"github.com/mondegor/go-core/mrlog"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/wire/mrauth/oplogger/collector"
)

// TestNewService - проверяет сборку графа коллектора журнала (репозиторий + usecase + MessageCollector).
// Конструирование не обращается к БД и не запускает воркеры, поэтому зависимости можно не поднимать.
func TestNewService(t *testing.T) {
	t.Parallel()

	svc := collector.NewService(
		nil, // DBConnManager: при конструировании не используется
		nil, // errorHandler
		mrlog.NopLogger(),
		nil, // traceManager
		"sample_schema.secure_operations_log",
		collector.WithMessageCollectorOpts(),
	)

	require.NotNil(t, svc)

	// коллектор выступает продюсером записей журнала (PushMessage) для usecase-ов
	var _ interface {
		PushMessage(ctx context.Context, entry entity.SecureOperationLog) error
	} = svc
}
