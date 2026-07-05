package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrtests/infra"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/tests"
)

const sessionsExcessQueueTableName = "sample_schema.sessions_excess_queue"

type SessionExcessQueuePostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *infra.PostgresTester
	repo *repository.SessionExcessQueuePostgres
}

func TestSessionExcessQueuePostgresTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SessionExcessQueuePostgresTestSuite))
}

func (ts *SessionExcessQueuePostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = infra.NewPostgresTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(tests.AppWorkDir() + "/mrauth/_sample/migrations")
	ts.repo = repository.NewSessionExcessQueuePostgres(ts.pgt.ConnManager(), sessionsExcessQueueTableName)
}

func (ts *SessionExcessQueuePostgresTestSuite) TearDownSuite() {
	ts.pgt.Destroy(ts.ctx)
}

func (ts *SessionExcessQueuePostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.ctx)
}

const (
	realmA uint16 = 1
	realmB uint16 = 2
)

// TestEnqueueFetchDelete - постановка пользователей в очередь, выборка пачки и ack обработанного.
func (ts *SessionExcessQueuePostgresTestSuite) TestEnqueueFetchDelete() {
	userA := uuid.New()
	userB := uuid.New()

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

// TestEnqueueUpsertUpdatesSessionMax - повтор по той же паре (user_id, realm) не дублирует строку
// и обновляет session_max значением последнего вызова.
func (ts *SessionExcessQueuePostgresTestSuite) TestEnqueueUpsertUpdatesSessionMax() {
	userID := uuid.New()

	ts.Require().NoError(ts.repo.Enqueue(ts.ctx, userID, realmA, 4))
	ts.Require().NoError(ts.repo.Enqueue(ts.ctx, userID, realmA, 7))

	items, err := ts.repo.Fetch(ts.ctx, 100)
	ts.Require().NoError(err)
	ts.Require().Len(items, 1)
	ts.Equal(entity.SessionExcessItem{UserID: userID, RealmID: realmA, SessionMax: 7}, items[0])
}

// TestEnqueueDistinctPerRealm - один пользователь в двух realm даёт две независимые строки очереди
// со своим session_max; ack одного realm не затрагивает другой.
func (ts *SessionExcessQueuePostgresTestSuite) TestEnqueueDistinctPerRealm() {
	userID := uuid.New()

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

// TestDeleteIdempotent - удаление отсутствующей пары (user_id, realm) не ошибка.
func (ts *SessionExcessQueuePostgresTestSuite) TestDeleteIdempotent() {
	ts.Require().NoError(ts.repo.Delete(ts.ctx, []entity.SessionExcessPK{{UserID: uuid.New(), RealmID: realmA}}))
}
