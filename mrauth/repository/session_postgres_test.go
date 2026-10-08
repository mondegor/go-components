package repository_test

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-storage/mrtests/pgtest"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/tests"
)

type SessionPostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *pgtest.Tester
	repo *repository.SessionPostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestSessionPostgresTestSuite(t *testing.T) {
	suite.Run(t, new(SessionPostgresTestSuite))
}

func (ts *SessionPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrauth"))

	ts.repo = repository.NewSessionPostgres(ts.pgt.ConnManager(), sessionsTableName)
}

func (ts *SessionPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// Test_Insert - вставленная сессия читается без искажений.
func (ts *SessionPostgresTestSuite) Test_Insert() {
	userID := uuid.MustParse(fixtureUserA)
	row := entity.Session{UserID: userID, SessionID: 42, UserAgent: "ua", LastIP: netip.MustParseAddr("203.0.113.7")}

	ts.Require().NoError(ts.repo.Insert(ts.ctx, row))

	rows, err := ts.repo.FetchOrderedListByUserIDAndSessionIDs(ts.ctx, userID, []uint32{42}, 0)
	ts.Require().NoError(err)
	ts.Require().Len(rows, 1)

	ts.Equal(userID, rows[0].UserID)
	ts.Equal(uint32(42), rows[0].SessionID)
	ts.Equal("ua", rows[0].UserAgent)
	ts.Equal(row.LastIP, rows[0].LastIP)
}

// Test_InsertWhenSessionIDCollision - повторная вставка той же пары (user_id, session_id)
// возвращает ErrEventRecordAlreadyExists, а не дублирует строку.
func (ts *SessionPostgresTestSuite) Test_InsertWhenSessionIDCollision() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Session/InsertWhenSessionIDCollision")

	row := entity.Session{UserID: uuid.MustParse(fixtureUserA), SessionID: 42, UserAgent: "ua", LastIP: netip.MustParseAddr("127.0.0.1")}

	err := ts.repo.Insert(ts.ctx, row)
	ts.Require().ErrorIs(err, errors.ErrEventRecordAlreadyExists)
	ts.Equal(1, ts.pgt.CountRows(ts.T(), ts.ctx, sessionsTableName))
}

// Test_FetchOrderedListByUserIDAndSessionIDs - выборка упорядочена активными вперёд (updated_at DESC,
// при равенстве - больший session_id вперёд), а положительный limit оставляет только новейшие.
func (ts *SessionPostgresTestSuite) Test_FetchOrderedListByUserIDAndSessionIDs() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Session/FetchOrderedListByUserIDAndSessionIDs")

	userID := uuid.MustParse(fixtureUserA)

	// без лимита: все три, новыми вперёд
	rows, err := ts.repo.FetchOrderedListByUserIDAndSessionIDs(ts.ctx, userID, []uint32{1, 2, 3}, 0)
	ts.Require().NoError(err)
	ts.Require().Len(rows, 3)
	ts.Equal([]uint32{3, 2, 1}, []uint32{rows[0].SessionID, rows[1].SessionID, rows[2].SessionID})

	// limit=2: только две новейшие
	rows, err = ts.repo.FetchOrderedListByUserIDAndSessionIDs(ts.ctx, userID, []uint32{1, 2, 3}, 2)
	ts.Require().NoError(err)
	ts.Require().Len(rows, 2)
	ts.Equal([]uint32{3, 2}, []uint32{rows[0].SessionID, rows[1].SessionID})
}

// Test_UpdateLastActivity - пакет обновляет IP и время последней активности сессии, только если
// активность новее сохранённой (запоздавшая запись не откатывает сессию назад); записи без
// совпадающей сессии игнорируются без ошибки.
func (ts *SessionPostgresTestSuite) Test_UpdateLastActivity() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Session/UpdateLastActivity")

	userID := uuid.MustParse(fixtureUserA)
	storedAt := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)

	err := ts.repo.UpdateLastActivity(ts.ctx, []dto.SessionLastActivity{
		{UserID: userID, SessionID: 1, LastIP: netip.MustParseAddr("198.51.100.9"), LastVisitedAt: storedAt.Add(time.Hour)},
		{UserID: userID, SessionID: 2, LastIP: netip.MustParseAddr("203.0.113.7"), LastVisitedAt: storedAt.Add(-time.Hour)},
		{UserID: userID, SessionID: 99, LastIP: netip.MustParseAddr("192.0.2.1"), LastVisitedAt: storedAt.Add(time.Hour)},
	})
	ts.Require().NoError(err)

	rows, err := ts.repo.FetchOrderedListByUserIDAndSessionIDs(ts.ctx, userID, []uint32{1, 2}, 0)
	ts.Require().NoError(err)
	ts.Require().Len(rows, 2)

	ts.Equal(uint32(1), rows[0].SessionID)
	ts.Equal(netip.MustParseAddr("198.51.100.9"), rows[0].LastIP)
	ts.WithinDuration(storedAt.Add(time.Hour), rows[0].UpdatedAt, time.Millisecond)

	ts.Equal(uint32(2), rows[1].SessionID)
	ts.Equal(netip.MustParseAddr("127.0.0.1"), rows[1].LastIP)
	ts.WithinDuration(storedAt, rows[1].UpdatedAt, time.Millisecond)

	ts.Equal(2, ts.pgt.CountRows(ts.T(), ts.ctx, sessionsTableName), "запись без сессии ничего не создаёт")
}

// Test_UpdateLastActivityWhenNoSessions - пакет, ни одна запись которого не имеет сессии, не ошибка
// и ничего не создаёт.
func (ts *SessionPostgresTestSuite) Test_UpdateLastActivityWhenNoSessions() {
	err := ts.repo.UpdateLastActivity(ts.ctx, []dto.SessionLastActivity{
		{UserID: uuid.MustParse(fixtureUserA), SessionID: 99, LastIP: netip.MustParseAddr("192.0.2.1"), LastVisitedAt: time.Now()},
	})
	ts.Require().NoError(err)
	ts.Equal(0, ts.pgt.CountRows(ts.T(), ts.ctx, sessionsTableName))
}

// Test_UpdateLastActivityWhenEmpty - пустой пакет не доходит до БД и не ошибка.
func (ts *SessionPostgresTestSuite) Test_UpdateLastActivityWhenEmpty() {
	ts.Require().NoError(ts.repo.UpdateLastActivity(ts.ctx, nil))
}
