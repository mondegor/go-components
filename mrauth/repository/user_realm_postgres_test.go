package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-storage/mrtests/pgtest"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/tests"
)

const usersRealmsTableName = "sample_schema.users_realms"

type UserRealmPostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *pgtest.Tester
	repo *repository.UserRealmPostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestUserRealmPostgresTestSuite(t *testing.T) {
	suite.Run(t, new(UserRealmPostgresTestSuite))
}

func (ts *UserRealmPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrauth"))

	ts.repo = repository.NewUserRealmPostgres(ts.pgt.ConnManager(), usersRealmsTableName)
}

func (ts *UserRealmPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// Test_Fetch - возвращаются привязки пользователя ко всем его realm в порядке realm_id;
// привязки других пользователей не попадают.
func (ts *UserRealmPostgresTestSuite) Test_Fetch() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/UserRealm/Fetch")

	userID := uuid.MustParse(fixtureUserA)
	createdAt := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)

	rows, err := ts.repo.Fetch(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Equal(
		[]entity.UserRealm{
			{UserID: userID, RealmID: realmA, Kind: "admin", CreatedAt: createdAt, UpdatedAt: updatedAt},
			{UserID: userID, RealmID: realmB, Kind: "standard", CreatedAt: createdAt, UpdatedAt: updatedAt},
		},
		rows,
	)
}

// Test_FetchWhenNoRows - у пользователя без привязок возвращается пустой срез, а не ошибка.
func (ts *UserRealmPostgresTestSuite) Test_FetchWhenNoRows() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/UserRealm/Fetch")

	rows, err := ts.repo.Fetch(ts.ctx, uuid.MustParse(fixtureUserC))
	ts.Require().NoError(err)
	ts.Empty(rows)
}

// Test_FetchOne - возвращается вид пользователя в указанном realm.
func (ts *UserRealmPostgresTestSuite) Test_FetchOne() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/UserRealm/FetchOne")

	userID := uuid.MustParse(fixtureUserA)

	row, err := ts.repo.FetchOne(ts.ctx, userID, realmA)
	ts.Require().NoError(err)
	ts.Equal(entity.UserRealm{UserID: userID, RealmID: realmA, Kind: "admin"}, row)
}

// Test_FetchOneWhenNotExists - без привязки к указанному realm возвращается ErrEventStorageNoRecordFound.
func (ts *UserRealmPostgresTestSuite) Test_FetchOneWhenNotExists() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/UserRealm/FetchOne")

	_, err := ts.repo.FetchOne(ts.ctx, uuid.MustParse(fixtureUserA), realmB)
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_Insert - привязка пользователя к realm сохраняется с указанным видом.
func (ts *UserRealmPostgresTestSuite) Test_Insert() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/UserRealm/Insert")

	userID := uuid.MustParse(fixtureUserA)

	ts.Require().NoError(ts.repo.Insert(ts.ctx, entity.UserRealm{UserID: userID, RealmID: realmA, Kind: "standard"}))

	row, err := ts.repo.FetchOne(ts.ctx, userID, realmA)
	ts.Require().NoError(err)
	ts.Equal("standard", row.Kind)
}

// Test_InsertWhenAlreadyExists - существующая привязка не перезаписывается:
// возвращается ErrEventRecordAlreadyExists, вид пользователя остаётся прежним.
func (ts *UserRealmPostgresTestSuite) Test_InsertWhenAlreadyExists() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/UserRealm/InsertWhenAlreadyExists")

	userID := uuid.MustParse(fixtureUserA)

	err := ts.repo.Insert(ts.ctx, entity.UserRealm{UserID: userID, RealmID: realmA, Kind: "standard"})
	ts.Require().ErrorIs(err, errors.ErrEventRecordAlreadyExists)

	row, err := ts.repo.FetchOne(ts.ctx, userID, realmA)
	ts.Require().NoError(err)
	ts.Equal("admin", row.Kind)
}

// Test_UpdateKind - меняется вид пользователя только в указанном realm, время обновления сдвигается.
func (ts *UserRealmPostgresTestSuite) Test_UpdateKind() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/UserRealm/UpdateKind")

	userID := uuid.MustParse(fixtureUserA)

	ts.Require().NoError(ts.repo.UpdateKind(ts.ctx, entity.UserRealm{UserID: userID, RealmID: realmA, Kind: "admin"}))

	rows, err := ts.repo.Fetch(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Require().Len(rows, 2)

	ts.Equal("admin", rows[0].Kind)
	ts.WithinDuration(time.Now(), rows[0].UpdatedAt, time.Minute)

	ts.Equal("standard", rows[1].Kind)
	ts.Equal(time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC), rows[1].UpdatedAt)
}

// Test_UpdateKindWhenNotExists - без привязки к указанному realm возвращается ErrEventStorageNoRecordFound.
func (ts *UserRealmPostgresTestSuite) Test_UpdateKindWhenNotExists() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/UserRealm/UpdateKind")

	err := ts.repo.UpdateKind(ts.ctx, entity.UserRealm{UserID: uuid.MustParse(fixtureUserB), RealmID: realmA, Kind: "admin"})
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_Delete - удаляется привязка только к указанному realm.
func (ts *UserRealmPostgresTestSuite) Test_Delete() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/UserRealm/Delete")

	userID := uuid.MustParse(fixtureUserA)

	ts.Require().NoError(ts.repo.Delete(ts.ctx, userID, realmA))

	rows, err := ts.repo.Fetch(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Require().Len(rows, 1)
	ts.Equal(realmB, rows[0].RealmID)
}

// Test_DeleteWhenNotExists - удаление отсутствующей привязки возвращает ErrEventStorageNoRecordFound.
func (ts *UserRealmPostgresTestSuite) Test_DeleteWhenNotExists() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/UserRealm/Delete")

	err := ts.repo.Delete(ts.ctx, uuid.MustParse(fixtureUserB), realmA)
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
	ts.Equal(2, ts.pgt.CountRows(ts.T(), ts.ctx, usersRealmsTableName))
}
