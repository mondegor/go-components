package repository_test

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrtype"
	"github.com/mondegor/go-storage/mrtests/pgtest"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/tests"
)

const usersActivityLogTableName = "sample_schema.users_activity_log"

type (
	UserActivityLogPostgresTestSuite struct {
		suite.Suite

		ctx  context.Context
		pgt  *pgtest.Tester
		repo *repository.UserActivityLogPostgres
	}

	// activityLogRow - строка журнала активности, считанная сырым SELECT (репозиторий read-метода не имеет).
	activityLogRow struct {
		UserID        uuid.UUID
		RealmID       uint16
		UserIP        netip.Addr
		UserProxyIP   netip.Addr
		UserAgent     string
		RequestPath   string
		RequestStatus uint32
		VisitedAt     time.Time
	}
)

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestUserActivityLogPostgresTestSuite(t *testing.T) {
	suite.Run(t, new(UserActivityLogPostgresTestSuite))
}

func (ts *UserActivityLogPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrauth"))

	ts.repo = repository.NewUserActivityLogPostgres(ts.pgt.ConnManager(), usersActivityLogTableName)
}

func (ts *UserActivityLogPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// Test_Insert - пачка записей пишется без искажений: real и proxy IP хранятся нативным inet
// (отсутствующий proxy - NULL), realm 0 допустим, время берётся из времени визита.
func (ts *UserActivityLogPostgresTestSuite) Test_Insert() {
	visitedAt := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)

	err := ts.repo.Insert(ts.ctx, []dto.UserActivityLogMessage{
		{
			UserID:        uuid.MustParse(fixtureUserA),
			RealmID:       realmA,
			UserIP:        mrtype.NewDetailedIP(netip.MustParseAddr("203.0.113.7"), netip.MustParseAddr("10.0.0.1")),
			UserAgent:     "Mozilla/5.0",
			RequestPath:   "/v1/user",
			RequestStatus: 200,
			VisitedAt:     visitedAt,
		},
		{
			UserID:        uuid.MustParse(fixtureUserB),
			RealmID:       0, // realm неизвестен
			UserIP:        mrtype.NewIP(netip.MustParseAddr("2001:db8::1")),
			RequestPath:   "/v1/admin",
			RequestStatus: 403,
			VisitedAt:     visitedAt.Add(time.Minute),
		},
	})
	ts.Require().NoError(err)

	ts.Equal(
		[]activityLogRow{
			{
				UserID:        uuid.MustParse(fixtureUserA),
				RealmID:       realmA,
				UserIP:        netip.MustParseAddr("203.0.113.7"),
				UserProxyIP:   netip.MustParseAddr("10.0.0.1"),
				UserAgent:     "Mozilla/5.0",
				RequestPath:   "/v1/user",
				RequestStatus: 200,
				VisitedAt:     visitedAt,
			},
			{
				UserID:        uuid.MustParse(fixtureUserB),
				UserIP:        netip.MustParseAddr("2001:db8::1"),
				RequestPath:   "/v1/admin",
				RequestStatus: 403,
				VisitedAt:     visitedAt.Add(time.Minute),
			},
		},
		ts.fetchRows(),
	)
}

// Test_InsertWhenEmpty - пустая пачка не доходит до БД и не ошибка.
func (ts *UserActivityLogPostgresTestSuite) Test_InsertWhenEmpty() {
	ts.Require().NoError(ts.repo.Insert(ts.ctx, nil))
}

// Test_InsertWhenUserIPUnset - незаданный real IP отвергается ограничением NOT NULL, а не пишется
// как NULL; вся пачка не записывается.
func (ts *UserActivityLogPostgresTestSuite) Test_InsertWhenUserIPUnset() {
	err := ts.repo.Insert(ts.ctx, []dto.UserActivityLogMessage{
		{UserID: uuid.MustParse(fixtureUserA), UserIP: mrtype.NewIP(netip.MustParseAddr("203.0.113.7")), RequestPath: "/v1/user"},
		{UserID: uuid.MustParse(fixtureUserA), UserIP: mrtype.DetailedIP{}, RequestPath: "/v1/user"}, // IP клиента не распознан
	})
	ts.Require().Error(err)
	ts.Empty(ts.fetchRows())
}

// Test_DeleteBeforeDate - записи старше границы удаляются пачками не более limit, начиная с самых
// старых; возвращается число удалённых строк, более новые записи остаются.
func (ts *UserActivityLogPostgresTestSuite) Test_DeleteBeforeDate() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/UserActivityLog/DeleteBeforeDate")

	border := time.Now().Add(-time.Hour)

	count, err := ts.repo.DeleteBeforeDate(ts.ctx, border, 1)
	ts.Require().NoError(err)
	ts.Equal(1, count)

	rows := ts.fetchRows()
	ts.Require().Len(rows, 2)
	ts.WithinDuration(time.Now().Add(-2*time.Hour), rows[0].VisitedAt, time.Minute, "самая старая запись удалена первой")

	count, err = ts.repo.DeleteBeforeDate(ts.ctx, border, 10)
	ts.Require().NoError(err)
	ts.Equal(1, count)

	count, err = ts.repo.DeleteBeforeDate(ts.ctx, border, 10)
	ts.Require().NoError(err)
	ts.Equal(0, count)

	rows = ts.fetchRows()
	ts.Require().Len(rows, 1)
	ts.True(rows[0].VisitedAt.After(border))
}

// fetchRows - все записи журнала в порядке времени визита.
func (ts *UserActivityLogPostgresTestSuite) fetchRows() []activityLogRow {
	rows, err := ts.pgt.ConnManager().Conn(ts.ctx).Query(
		ts.ctx,
		`SELECT user_id, realm_id, user_ip, user_proxy_ip, user_agent, request_path, request_status, visited_at
		 FROM `+usersActivityLogTableName+`
		 ORDER BY visited_at;`,
	)
	ts.Require().NoError(err)

	defer rows.Close()

	var out []activityLogRow

	for rows.Next() {
		var r activityLogRow

		ts.Require().NoError(rows.Scan(
			&r.UserID, &r.RealmID, &r.UserIP, &r.UserProxyIP, &r.UserAgent, &r.RequestPath, &r.RequestStatus, &r.VisitedAt,
		))

		r.VisitedAt = r.VisitedAt.UTC()

		out = append(out, r)
	}

	ts.Require().NoError(rows.Err())

	return out
}
