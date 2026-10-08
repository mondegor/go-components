package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrtests/pgtest"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/tests"
)

const sessionsCleanupQueueTableName = "sample_schema.sessions_cleanup_queue"

type SessionCleanupQueuePostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *pgtest.Tester
	repo *repository.SessionCleanupQueuePostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestSessionCleanupQueuePostgresTestSuite(t *testing.T) {
	suite.Run(t, new(SessionCleanupQueuePostgresTestSuite))
}

func (ts *SessionCleanupQueuePostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrauth"))

	ts.repo = repository.NewSessionCleanupQueuePostgres(ts.pgt.ConnManager(), sessionsCleanupQueueTableName)
}

func (ts *SessionCleanupQueuePostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// Test_Enqueue - пары ставятся в очередь, повтор внутри одного пакета не дублирует строку.
func (ts *SessionCleanupQueuePostgresTestSuite) Test_Enqueue() {
	userA := uuid.MustParse(fixtureUserA)
	userB := uuid.MustParse(fixtureUserB)

	err := ts.repo.Enqueue(ts.ctx, []entity.SessionPK{
		{UserID: userA, SessionID: 1},
		{UserID: userB, SessionID: 2},
		{UserID: userA, SessionID: 1},
	})
	ts.Require().NoError(err)

	pks, err := ts.repo.Fetch(ts.ctx, 100)
	ts.Require().NoError(err)
	ts.ElementsMatch([]entity.SessionPK{{UserID: userA, SessionID: 1}, {UserID: userB, SessionID: 2}}, pks)
}

// Test_EnqueueWhenAlreadyQueued - пара, уже стоящая в очереди, не дублируется и не теряет
// своего места в очереди (время постановки не меняется).
func (ts *SessionCleanupQueuePostgresTestSuite) Test_EnqueueWhenAlreadyQueued() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/SessionCleanupQueue/EnqueueWhenAlreadyQueued")

	userID := uuid.MustParse(fixtureUserA)

	ts.Require().NoError(ts.repo.Enqueue(ts.ctx, []entity.SessionPK{{UserID: userID, SessionID: 1}}))
	ts.Equal(1, ts.pgt.CountRows(ts.T(), ts.ctx, sessionsCleanupQueueTableName))

	var createdAt time.Time

	err := ts.pgt.ConnManager().Conn(ts.ctx).QueryRow(
		ts.ctx,
		`SELECT created_at FROM `+sessionsCleanupQueueTableName+` WHERE user_id = $1 AND session_id = $2;`,
		userID,
		1,
	).Scan(&createdAt)
	ts.Require().NoError(err)
	ts.Equal(time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC), createdAt.UTC())
}

// Test_EnqueueWhenEmpty - пустой пакет не доходит до БД и не ошибка.
func (ts *SessionCleanupQueuePostgresTestSuite) Test_EnqueueWhenEmpty() {
	ts.Require().NoError(ts.repo.Enqueue(ts.ctx, nil))
}

// Test_Fetch - выбирается не более limit пар, раньше поставленные в очередь первыми.
func (ts *SessionCleanupQueuePostgresTestSuite) Test_Fetch() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/SessionCleanupQueue/Fetch")

	pks, err := ts.repo.Fetch(ts.ctx, 2)
	ts.Require().NoError(err)
	ts.Equal(
		[]entity.SessionPK{
			{UserID: uuid.MustParse(fixtureUserB), SessionID: 3},
			{UserID: uuid.MustParse(fixtureUserA), SessionID: 1},
		},
		pks,
	)
}

// Test_FetchWhenEmpty - из пустой очереди выбирается пустой срез, а не ошибка.
func (ts *SessionCleanupQueuePostgresTestSuite) Test_FetchWhenEmpty() {
	pks, err := ts.repo.Fetch(ts.ctx, 100)
	ts.Require().NoError(err)
	ts.Empty(pks)
}

// Test_Delete - из очереди удаляются только указанные пары.
func (ts *SessionCleanupQueuePostgresTestSuite) Test_Delete() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/SessionCleanupQueue/Delete")

	userA := uuid.MustParse(fixtureUserA)

	ts.Require().NoError(ts.repo.Delete(ts.ctx, []entity.SessionPK{
		{UserID: userA, SessionID: 1},
		{UserID: uuid.MustParse(fixtureUserB), SessionID: 3},
	}))

	pks, err := ts.repo.Fetch(ts.ctx, 100)
	ts.Require().NoError(err)
	ts.Equal([]entity.SessionPK{{UserID: userA, SessionID: 2}}, pks)
}

// Test_DeleteWhenNotQueued - удаление отсутствующих в очереди пар не ошибка (ack идемпотентен).
func (ts *SessionCleanupQueuePostgresTestSuite) Test_DeleteWhenNotQueued() {
	ts.Require().NoError(ts.repo.Delete(ts.ctx, []entity.SessionPK{{UserID: uuid.MustParse(fixtureUserA), SessionID: 1}}))
}
