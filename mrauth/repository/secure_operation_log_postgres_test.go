package repository_test

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrtype"
	"github.com/mondegor/go-storage/mrtests/infra"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/logreason"
	"github.com/mondegor/go-components/mrauth/enum/logstatus"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/tests"
)

const secureOperationsLogTableName = "sample_schema.secure_operations_log"

// logRow - строка журнала, считанная сырым SELECT для проверки записи (репозиторий read-метода не имеет).
type logRow struct {
	VisitorID     uuid.UUID
	OperationName string
	ConfirmMethod int16
	LogStatus     int16
	Reason        int16
	ClientIP      netip.Addr
	ClientProxyIP netip.Addr
	CreatedAt     time.Time
}

type SecureOperationLogPostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *infra.PostgresTester
	repo *repository.SecureOperationLogPostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestSecureOperationLogPostgresTestSuite(t *testing.T) {
	suite.Run(t, new(SecureOperationLogPostgresTestSuite))
}

func (ts *SecureOperationLogPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = infra.NewPostgresTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(tests.AppWorkDir() + "/mrauth/_sample/migrations")
	ts.repo = repository.NewSecureOperationLogPostgres(ts.pgt.ConnManager(), secureOperationsLogTableName)
}

func (ts *SecureOperationLogPostgresTestSuite) TearDownSuite() {
	ts.pgt.Destroy(ts.ctx)
}

func (ts *SecureOperationLogPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.ctx)
}

// fetchAll - сырой SELECT всех записей журнала в порядке record_id.
func (ts *SecureOperationLogPostgresTestSuite) fetchAll() []logRow {
	rows, err := ts.pgt.ConnManager().Conn(ts.ctx).Query(
		ts.ctx,
		`SELECT visitor_id, operation_name, confirm_method, log_status, reason, client_ip, client_proxy_ip, created_at
		 FROM `+secureOperationsLogTableName+`
		 ORDER BY record_id`,
	)
	ts.Require().NoError(err)

	defer rows.Close()

	var out []logRow

	for rows.Next() {
		var r logRow

		ts.Require().NoError(rows.Scan(
			&r.VisitorID, &r.OperationName, &r.ConfirmMethod, &r.LogStatus, &r.Reason,
			&r.ClientIP, &r.ClientProxyIP, &r.CreatedAt,
		))

		out = append(out, r)
	}

	ts.Require().NoError(rows.Err())

	return out
}

// TestInsertEmptyNoop - пустой батч не выполняет запрос и не создаёт строк.
func (ts *SecureOperationLogPostgresTestSuite) TestInsertEmptyNoop() {
	ts.Require().NoError(ts.repo.Insert(ts.ctx, nil))
	ts.Empty(ts.fetchAll())
}

// TestInsertRoundTrip - батч из залогиненного и анонимного событий пишется и читается без искажений
// (проверяет корректность UNNEST-приведений uuid/int2/inet/timestamptz, хранение real/proxy IP
// нативным inet и то, что created_at берётся из времени события, а не из времени вставки пачки).
func (ts *SecureOperationLogPostgresTestSuite) TestInsertRoundTrip() {
	visitor := uuid.New()
	eventAt := time.Now().Add(-2 * time.Hour)

	rows := []entity.SecureOperationLog{
		{
			VisitorID:     visitor,
			OperationName: "confirm.authorize.user",
			ConfirmMethod: confirmmethod.Email,
			LogStatus:     logstatus.Confirmed,
			Reason:        logreason.Unspecified,
			ClientIP:      mrtype.NewDetailedIP(netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("10.0.0.1")),
			CreatedAt:     eventAt,
		},
		{
			VisitorID:     uuid.Nil, // анонимный поток
			OperationName: "session.continue",
			ConfirmMethod: confirmmethod.Unspecified,
			LogStatus:     logstatus.Blocked,
			Reason:        logreason.TokenReuse,
			// анонимный поток тоже имеет real IP (RemoteAddr), но приходит без прокси
			ClientIP:  mrtype.NewIP(netip.MustParseAddr("198.51.100.9")),
			CreatedAt: eventAt,
		},
	}

	ts.Require().NoError(ts.repo.Insert(ts.ctx, rows))

	got := ts.fetchAll()
	ts.Require().Len(got, 2)

	ts.Equal(visitor, got[0].VisitorID)
	ts.Equal("confirm.authorize.user", got[0].OperationName)
	ts.Equal(int16(confirmmethod.Email), got[0].ConfirmMethod)
	ts.Equal(int16(logstatus.Confirmed), got[0].LogStatus)
	ts.Equal(int16(logreason.Unspecified), got[0].Reason)
	ts.Equal(netip.MustParseAddr("127.0.0.1"), got[0].ClientIP)
	ts.Equal(netip.MustParseAddr("10.0.0.1"), got[0].ClientProxyIP)
	ts.WithinDuration(eventAt, got[0].CreatedAt, time.Millisecond)

	ts.Equal(uuid.Nil, got[1].VisitorID)
	ts.Equal("session.continue", got[1].OperationName)
	ts.Equal(int16(confirmmethod.Unspecified), got[1].ConfirmMethod)
	ts.Equal(int16(logstatus.Blocked), got[1].LogStatus)
	ts.Equal(int16(logreason.TokenReuse), got[1].Reason)
	ts.Equal(netip.MustParseAddr("198.51.100.9"), got[1].ClientIP)
	ts.False(got[1].ClientProxyIP.IsValid()) // proxy отсутствует -> NULL
	ts.WithinDuration(eventAt, got[1].CreatedAt, time.Millisecond)
}

// TestInsertRejectsUnsetClientIP - незаданный real IP отвергается ограничением NOT NULL,
// а не пишется как NULL: client_ip берётся из RemoteAddr и известен для любого запроса, включая
// анонимные потоки (pgx кодирует невалидный netip.Addr как NULL - без ограничения он утёк бы в БД).
func (ts *SecureOperationLogPostgresTestSuite) TestInsertRejectsUnsetClientIP() {
	rows := []entity.SecureOperationLog{
		entity.NewSecureOperationLog(
			uuid.New(),
			mrtype.DetailedIP{}, // IP клиента не распознан
			"confirm.authorize.user",
			confirmmethod.Email,
			logstatus.Opened,
			logreason.Unspecified,
		),
	}

	ts.Require().Error(ts.repo.Insert(ts.ctx, rows))
	ts.Empty(ts.fetchAll())
}

// TestInsertIPv6StoredNatively - IPv6-адрес хранится нативным inet наравне с IPv4
// (в одной пачке с IPv4-записями, без потерь и искажений).
func (ts *SecureOperationLogPostgresTestSuite) TestInsertIPv6StoredNatively() {
	rows := []entity.SecureOperationLog{
		entity.NewSecureOperationLog(
			uuid.New(),
			mrtype.NewIP(netip.MustParseAddr("127.0.0.1")),
			"confirm.authorize.user",
			confirmmethod.Email,
			logstatus.Opened,
			logreason.Unspecified,
		),
		entity.NewSecureOperationLog(
			uuid.Nil,
			mrtype.NewIP(netip.MustParseAddr("2001:db8::1")),
			"confirm.authorize.user",
			confirmmethod.Unspecified,
			logstatus.Blocked,
			logreason.LoginNotExists,
		),
		entity.NewSecureOperationLog(
			uuid.New(),
			mrtype.NewIP(netip.MustParseAddr("192.0.2.1")),
			"confirm.create.user",
			confirmmethod.Email,
			logstatus.Opened,
			logreason.Unspecified,
		),
	}

	ts.Require().NoError(ts.repo.Insert(ts.ctx, rows))

	got := ts.fetchAll()
	ts.Require().Len(got, 3) // соседние записи пачки не потеряны

	ts.Equal(netip.MustParseAddr("127.0.0.1"), got[0].ClientIP)

	ts.Equal(netip.MustParseAddr("2001:db8::1"), got[1].ClientIP) // IPv6 хранится нативно
	ts.Equal(int16(logreason.LoginNotExists), got[1].Reason)

	ts.Equal(netip.MustParseAddr("192.0.2.1"), got[2].ClientIP)

	// время события проставляется конструктором записи
	ts.WithinDuration(time.Now(), got[0].CreatedAt, time.Minute)
}

// TestDeleteBeforeDate - прунинг удаляет записи старше границы created_at пачками по limit,
// не трогая более новые.
func (ts *SecureOperationLogPostgresTestSuite) TestDeleteBeforeDate() {
	rows := make([]entity.SecureOperationLog, 0, 3)
	for range 3 {
		rows = append(
			rows,
			entity.NewSecureOperationLog(
				uuid.New(),
				mrtype.NewIP(netip.MustParseAddr("127.0.0.1")),
				"confirm.authorize.user",
				confirmmethod.Email,
				logstatus.Opened,
				logreason.Unspecified,
			),
		)
	}

	ts.Require().NoError(ts.repo.Insert(ts.ctx, rows))

	// граница в прошлом: ничего не удаляется (все записи только что созданы)
	count, err := ts.repo.DeleteBeforeDate(ts.ctx, time.Now().Add(-time.Hour), 100)
	ts.Require().NoError(err)
	ts.Equal(0, count)
	ts.Len(ts.fetchAll(), 3)

	// граница в будущем + limit=2: удаляется ровно одна полная пачка
	count, err = ts.repo.DeleteBeforeDate(ts.ctx, time.Now().Add(time.Hour), 2)
	ts.Require().NoError(err)
	ts.Equal(2, count)
	ts.Len(ts.fetchAll(), 1)

	// остаток удаляется следующим вызовом
	count, err = ts.repo.DeleteBeforeDate(ts.ctx, time.Now().Add(time.Hour), 2)
	ts.Require().NoError(err)
	ts.Equal(1, count)
	ts.Empty(ts.fetchAll())
}
