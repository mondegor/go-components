package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-storage/mrtests/pgtest"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/tests"
)

type CheckUserPostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *pgtest.Tester
	repo *repository.CheckUserPostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestCheckUserPostgresTestSuite(t *testing.T) {
	suite.Run(t, new(CheckUserPostgresTestSuite))
}

func (ts *CheckUserPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrauth"))

	ts.repo = repository.NewCheckUserPostgres(ts.pgt.ConnManager(), usersTableName)
}

func (ts *CheckUserPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// Test_UserIDByEmail - по email находится идентификатор пользователя.
func (ts *CheckUserPostgresTestSuite) Test_UserIDByEmail() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/CheckUser/UserIDByEmail")

	userID, err := ts.repo.UserIDByEmail(ts.ctx, "user-a@localhost")
	ts.Require().NoError(err)
	ts.Equal(uuid.MustParse(fixtureUserA), userID)
}

// Test_UserIDByEmailWhenUserDeleted - email мягко удалённого пользователя не находится:
// ErrEventStorageNoRecordFound.
func (ts *CheckUserPostgresTestSuite) Test_UserIDByEmailWhenUserDeleted() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/CheckUser/UserIDByEmail")

	_, err := ts.repo.UserIDByEmail(ts.ctx, "user-b@localhost")
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_UserIDByEmailWhenNotExists - неизвестный email возвращает ErrEventStorageNoRecordFound.
func (ts *CheckUserPostgresTestSuite) Test_UserIDByEmailWhenNotExists() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/CheckUser/UserIDByEmail")

	_, err := ts.repo.UserIDByEmail(ts.ctx, "unknown@localhost")
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_UserIDByPhone - по телефону находится идентификатор пользователя.
func (ts *CheckUserPostgresTestSuite) Test_UserIDByPhone() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/CheckUser/UserIDByPhone")

	userID, err := ts.repo.UserIDByPhone(ts.ctx, 9876543210)
	ts.Require().NoError(err)
	ts.Equal(uuid.MustParse(fixtureUserA), userID)
}

// Test_UserIDByPhoneWhenUserDeleted - телефон мягко удалённого пользователя не находится:
// ErrEventStorageNoRecordFound.
func (ts *CheckUserPostgresTestSuite) Test_UserIDByPhoneWhenUserDeleted() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/CheckUser/UserIDByPhone")

	_, err := ts.repo.UserIDByPhone(ts.ctx, 9000000002)
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_UserIDByPhoneWhenNotExists - неизвестный телефон возвращает ErrEventStorageNoRecordFound.
func (ts *CheckUserPostgresTestSuite) Test_UserIDByPhoneWhenNotExists() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/CheckUser/UserIDByPhone")

	_, err := ts.repo.UserIDByPhone(ts.ctx, 9000000001)
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}
