package collect_test

import (
	"context"
	"testing"
	"time"

	"github.com/mondegor/go-core/mrlog"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/infra/adapter/collect"
	"github.com/mondegor/go-components/mrauth/infra/adapter/collect/mock"
)

//go:generate mockgen -source=secure_operation.go -destination=mock/secure_operation.go -package=mock

type SecureOperationLoggerSuite struct {
	suite.Suite

	ctrl     *gomock.Controller
	producer *mock.MocksecureOperationLogProducer
	entries  []entity.SecureOperationLog
}

func TestSecureOperationLoggerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SecureOperationLoggerSuite))
}

func (s *SecureOperationLoggerSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.producer = mock.NewMocksecureOperationLogProducer(s.ctrl)
	s.entries = nil
}

// expectAccepts - продюсер принимает записи журнала.
func (s *SecureOperationLoggerSuite) expectAccepts() {
	s.producer.EXPECT().
		PushMessage(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, entry entity.SecureOperationLog) error {
			s.entries = append(s.entries, entry)

			return nil
		}).
		AnyTimes()
}

// expectBlocked - продюсер имитирует переполненную очередь коллектора:
// PushMessage блокируется до истечения переданного контекста.
func (s *SecureOperationLoggerSuite) expectBlocked() {
	s.producer.EXPECT().
		PushMessage(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ entity.SecureOperationLog) error {
			<-ctx.Done()

			return ctx.Err()
		}).
		AnyTimes()
}

func (s *SecureOperationLoggerSuite) TestLogPushesToCollector() {
	s.expectAccepts()

	logger := collect.NewSecureOperationLogger(s.producer, mrlog.NopLogger())
	logger.Log(context.Background(), entity.SecureOperationLog{SourceName: "AUTHORIZE_USER"})

	s.Require().Len(s.entries, 1)
	s.Equal("AUTHORIZE_USER", s.entries[0].SourceName)
}

// TestLogOverflowDoesNotBlockLong - журнал best-effort: при заторе в коллекторе
// вызов не должен надолго удерживать горутину запроса.
func (s *SecureOperationLoggerSuite) TestLogOverflowDoesNotBlockLong() {
	s.expectBlocked()

	logger := collect.NewSecureOperationLogger(s.producer, mrlog.NopLogger())

	start := time.Now()

	logger.Log(context.Background(), entity.SecureOperationLog{SourceName: "AUTHORIZE_USER"})

	s.Less(time.Since(start), time.Second, "ожидание места в очереди ограничено pushTimeout")
}

// TestLogNilProducerDisabledNoPanic - хост может не поднимать коллектор журнала:
// вместо паники на nil-продюсере журнал молча отключается.
func (s *SecureOperationLoggerSuite) TestLogNilProducerDisabledNoPanic() {
	logger := collect.NewSecureOperationLogger(nil, mrlog.NopLogger())

	s.NotPanics(func() {
		logger.Log(context.Background(), entity.SecureOperationLog{SourceName: "AUTHORIZE_USER"})
	})
}

// TestLogCanceledRequestStillLogs - отмена запроса не должна отменять запись:
// контекст продюсера отвязан от контекста запроса.
func (s *SecureOperationLoggerSuite) TestLogCanceledRequestStillLogs() {
	s.expectAccepts()

	logger := collect.NewSecureOperationLogger(s.producer, mrlog.NopLogger())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	logger.Log(ctx, entity.SecureOperationLog{SourceName: "AUTHORIZE_USER"})

	s.Require().Len(s.entries, 1)
}
