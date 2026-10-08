package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrtests/pgtest"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/tests"
)

const sessionsExcessQueueTableName = "sample_schema.sessions_excess_queue"

type SessionExcessQueuePostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *pgtest.Tester
	repo *repository.SessionExcessQueuePostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestSessionExcessQueuePostgresTestSuite(t *testing.T) {
	suite.Run(t, new(SessionExcessQueuePostgresTestSuite))
}

func (ts *SessionExcessQueuePostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrauth"))
	ts.repo = repository.NewSessionExcessQueuePostgres(ts.pgt.ConnManager(), sessionsExcessQueueTableName)
}

func (ts *SessionExcessQueuePostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// Test_Enqueue - постановка пользователей в очередь, выборка пачки и ack обработанного.
func (ts *SessionExcessQueuePostgresTestSuite) Test_Enqueue() {
	userA := uuid.MustParse(fixtureUserA)
	userB := uuid.MustParse(fixtureUserB)

	ts.Require().NoError(ts.repo.Enqueue(ts.ctx, userA, realmA, 4))
	ts.Require().NoError(ts.repo.Enqueue(ts.ctx, userB, realmA, 8))

	items, err := ts.repo.Fetch(ts.ctx, 100)
	ts.Require().NoError(err)
	ts.Require().Len(items, 2)
	// порядок - по created_at: userA поставлен раньше userB
	ts.Equal(entity.SessionExcessItem{UserID: userA, RealmID: realmA, SessionMax: 4}, items[0])
	ts.Equal(entity.SessionExcessItem{UserID: userB, RealmID: realmA, SessionMax: 8}, items[1])

	ts.Require().NoError(ts.repo.Delete(ts.ctx, []entity.SessionExcessPK{{UserID: userA, RealmID: realmA}}))

	items, err = ts.repo.Fetch(ts.ctx, 100)
	ts.Require().NoError(err)
	ts.Require().Len(items, 1)
	ts.Equal(userB, items[0].UserID)
}

// Test_EnqueueWhenAlreadyQueued - повтор по той же паре (user_id, realm) не дублирует строку
// и обновляет session_max значением последнего вызова.
func (ts *SessionExcessQueuePostgresTestSuite) Test_EnqueueWhenAlreadyQueued() {
	userID := uuid.MustParse(fixtureUserA)

	ts.Require().NoError(ts.repo.Enqueue(ts.ctx, userID, realmA, 4))
	ts.Require().NoError(ts.repo.Enqueue(ts.ctx, userID, realmA, 7))

	items, err := ts.repo.Fetch(ts.ctx, 100)
	ts.Require().NoError(err)
	ts.Require().Len(items, 1)
	ts.Equal(entity.SessionExcessItem{UserID: userID, RealmID: realmA, SessionMax: 7}, items[0])
}

// Test_EnqueueWhenOtherRealm - один пользователь в двух realm даёт две независимые строки очереди
// со своим session_max; ack одного realm не затрагивает другой.
func (ts *SessionExcessQueuePostgresTestSuite) Test_EnqueueWhenOtherRealm() {
	userID := uuid.MustParse(fixtureUserA)

	ts.Require().NoError(ts.repo.Enqueue(ts.ctx, userID, realmA, 4))
	ts.Require().NoError(ts.repo.Enqueue(ts.ctx, userID, realmB, 2))

	items, err := ts.repo.Fetch(ts.ctx, 100)
	ts.Require().NoError(err)
	ts.Require().Len(items, 2)
	ts.Equal(entity.SessionExcessItem{UserID: userID, RealmID: realmA, SessionMax: 4}, items[0])
	ts.Equal(entity.SessionExcessItem{UserID: userID, RealmID: realmB, SessionMax: 2}, items[1])

	// ack только realmA - строка realmB остаётся в очереди
	ts.Require().NoError(ts.repo.Delete(ts.ctx, []entity.SessionExcessPK{{UserID: userID, RealmID: realmA}}))

	items, err = ts.repo.Fetch(ts.ctx, 100)
	ts.Require().NoError(err)
	ts.Require().Len(items, 1)
	ts.Equal(entity.SessionExcessItem{UserID: userID, RealmID: realmB, SessionMax: 2}, items[0])
}

// Test_DeleteWhenNotQueued - удаление отсутствующей пары (user_id, realm) не ошибка.
func (ts *SessionExcessQueuePostgresTestSuite) Test_DeleteWhenNotQueued() {
	ts.Require().NoError(ts.repo.Delete(ts.ctx, []entity.SessionExcessPK{{UserID: uuid.MustParse(fixtureUserC), RealmID: realmA}}))
}

// Test_Fetch - выбирается не более limit записей, раньше поставленные в очередь первыми.
func (ts *SessionExcessQueuePostgresTestSuite) Test_Fetch() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/SessionExcessQueue/Fetch")

	items, err := ts.repo.Fetch(ts.ctx, 2)
	ts.Require().NoError(err)
	ts.Equal(
		[]entity.SessionExcessItem{
			{UserID: uuid.MustParse(fixtureUserB), RealmID: realmA, SessionMax: 8},
			{UserID: uuid.MustParse(fixtureUserA), RealmID: realmA, SessionMax: 6},
		},
		items,
	)
}

// Test_FetchWhenEmpty - из пустой очереди выбирается пустой срез, а не ошибка.
func (ts *SessionExcessQueuePostgresTestSuite) Test_FetchWhenEmpty() {
	items, err := ts.repo.Fetch(ts.ctx, 100)
	ts.Require().NoError(err)
	ts.Empty(items)
}
