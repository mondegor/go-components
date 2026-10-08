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

type OrphanSessionDeleterPostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *pgtest.Tester
	repo *repository.OrphanSessionDeleterPostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestOrphanSessionDeleterPostgresTestSuite(t *testing.T) {
	suite.Run(t, new(OrphanSessionDeleterPostgresTestSuite))
}

func (ts *OrphanSessionDeleterPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrauth"))

	ts.repo = repository.NewOrphanSessionDeleterPostgres(ts.pgt.ConnManager(), sessionsTableName, authTokensTableName)
}

func (ts *OrphanSessionDeleterPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// Test_DeleteOrphaned - удаляются только осиротевшие сессии (без живого ENABLED непросроченного
// refresh-токена); живая сессия (в т.ч. после ротации) сохраняется.
func (ts *OrphanSessionDeleterPostgresTestSuite) Test_DeleteOrphaned() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/OrphanSessionDeleter/DeleteOrphaned")

	userID := uuid.MustParse(fixtureUserA)

	candidates := []entity.SessionPK{
		{UserID: userID, SessionID: 1},
		{UserID: userID, SessionID: 2},
		{UserID: userID, SessionID: 3},
		{UserID: userID, SessionID: 4},
	}

	ts.Require().NoError(ts.repo.DeleteOrphaned(ts.ctx, candidates))
	ts.Equal([]uint32{2}, ts.fetchSessionIDs(userID)) // осталась только живая сессия
}

// fetchSessionIDs - идентификаторы сессий пользователя в порядке возрастания.
func (ts *OrphanSessionDeleterPostgresTestSuite) fetchSessionIDs(userID uuid.UUID) []uint32 {
	rows, err := ts.pgt.ConnManager().Conn(ts.ctx).Query(
		ts.ctx,
		`SELECT session_id FROM `+sessionsTableName+` WHERE user_id = $1 ORDER BY session_id;`,
		userID,
	)
	ts.Require().NoError(err)

	defer rows.Close()

	var sessionIDs []uint32

	for rows.Next() {
		var sessionID uint32

		ts.Require().NoError(rows.Scan(&sessionID))

		sessionIDs = append(sessionIDs, sessionID)
	}

	ts.Require().NoError(rows.Err())

	return sessionIDs
}
