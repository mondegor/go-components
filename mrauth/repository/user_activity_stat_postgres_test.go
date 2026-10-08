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

const usersActivityStatTableName = "sample_schema.users_activity_stat"

type UserActivityStatPostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *pgtest.Tester
	repo *repository.UserActivityStatPostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestUserActivityStatPostgresTestSuite(t *testing.T) {
	suite.Run(t, new(UserActivityStatPostgresTestSuite))
}

func (ts *UserActivityStatPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrauth"))
	ts.repo = repository.NewUserActivityStatPostgres(ts.pgt.ConnManager(), usersActivityStatTableName)
}

func (ts *UserActivityStatPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// baseTime - опорное время тестов (совпадает со временем в фикстурах) без наносекунд: timestamptz хранит микросекунды.
func (ts *UserActivityStatPostgresTestSuite) baseTime() time.Time {
	return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
}

// stat - собирает строку статистики с указанным realm и IP.
func (ts *UserActivityStatPostgresTestSuite) stat(userID uuid.UUID, realmID uint16, ip string) entity.UserActivityStat {
	return entity.UserActivityStat{
		UserID:        userID,
		RealmID:       realmID,
		LastLoginIP:   netip.MustParseAddr(ip),
		LastLoggedAt:  ts.baseTime(),
		LastVisitedAt: ts.baseTime(),
	}
}

// Test_Fetch - статистика пользователя выбирается по всем его realm'ам в порядке realm_id.
func (ts *UserActivityStatPostgresTestSuite) Test_Fetch() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/UserActivityStat/Fetch")

	userID := uuid.MustParse(fixtureUserA)

	rows, err := ts.repo.Fetch(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Require().Len(rows, 2)

	ts.Equal(realmA, rows[0].RealmID)
	ts.Equal(userID, rows[0].UserID)
	ts.Equal("203.0.113.7", rows[0].LastLoginIP.String())
	ts.WithinDuration(ts.baseTime(), rows[0].LastLoggedAt, time.Millisecond)

	ts.Equal(realmB, rows[1].RealmID)
	ts.Equal("198.51.100.9", rows[1].LastLoginIP.String())
}

// Test_FetchWhenNoRows - у пользователя без статистики Fetch возвращает пустой срез, а не ошибку.
func (ts *UserActivityStatPostgresTestSuite) Test_FetchWhenNoRows() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/UserActivityStat/FetchWhenNoRows")

	userID := uuid.MustParse(fixtureUserA)

	rows, err := ts.repo.Fetch(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Empty(rows)
}

// Test_InsertOrUpdateWhenLoginIPUnset - незаданный IP входа отвергается ограничением NOT NULL,
// а не записывается как NULL: строка статистики заводится только при входе, поэтому IP входа
// известен всегда (pgx кодирует невалидный netip.Addr как NULL - без ограничения он утёк бы в БД).
// Вызывающий такой сбой не проваливает: запись активности best-effort, см. OpenSession.Execute.
func (ts *UserActivityStatPostgresTestSuite) Test_InsertOrUpdateWhenLoginIPUnset() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/UserActivityStat/InsertOrUpdateWhenLoginIPUnset")

	userID := uuid.MustParse(fixtureUserA)

	row := ts.stat(userID, realmA, "203.0.113.7")
	row.LastLoginIP = netip.Addr{} // IP клиента не распознан

	ts.Require().Error(ts.repo.InsertOrUpdate(ts.ctx, row))

	// строки нет вовсе: частичная запись без IP не создаётся
	rows, err := ts.repo.Fetch(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Empty(rows)
}

// Test_InsertOrUpdateWhenExists - повторный вызов для той же пары (user, realm) обновляет строку.
func (ts *UserActivityStatPostgresTestSuite) Test_InsertOrUpdateWhenExists() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/UserActivityStat/InsertOrUpdateWhenExists")

	userID := uuid.MustParse(fixtureUserA)

	updated := ts.stat(userID, realmA, "198.51.100.9")
	updated.LastLoggedAt = ts.baseTime().Add(time.Hour)
	updated.LastVisitedAt = ts.baseTime().Add(time.Hour)
	ts.Require().NoError(ts.repo.InsertOrUpdate(ts.ctx, updated))

	rows, err := ts.repo.Fetch(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Require().Len(rows, 1)
	ts.Equal("198.51.100.9", rows[0].LastLoginIP.String())
	ts.WithinDuration(ts.baseTime().Add(time.Hour), rows[0].LastLoggedAt, time.Millisecond)
}

// Test_InsertOrUpdate - строки разных realm'ов одного пользователя не затирают друг друга.
func (ts *UserActivityStatPostgresTestSuite) Test_InsertOrUpdate() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/UserActivityStat/InsertOrUpdate")

	userID := uuid.MustParse(fixtureUserA)

	ts.Require().NoError(ts.repo.InsertOrUpdate(ts.ctx, ts.stat(userID, realmA, "203.0.113.7")))
	ts.Require().NoError(ts.repo.InsertOrUpdate(ts.ctx, ts.stat(userID, realmB, "198.51.100.9")))

	rows, err := ts.repo.Fetch(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Require().Len(rows, 2)
	ts.Equal("203.0.113.7", rows[0].LastLoginIP.String())
	ts.Equal("198.51.100.9", rows[1].LastLoginIP.String())
}

// Test_UpdateLastVisited - пакет обновляет last_visited_at строго по паре (user, realm).
func (ts *UserActivityStatPostgresTestSuite) Test_UpdateLastVisited() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/UserActivityStat/UpdateLastVisited")

	userID := uuid.MustParse(fixtureUserA)

	visited := ts.baseTime().Add(time.Hour)

	// обновляется только realm A: realm B того же пользователя должен остаться нетронутым
	err := ts.repo.UpdateLastVisited(ts.ctx, []dto.UserActivityLastVisited{
		{UserID: userID, RealmID: realmA, LastVisitedAt: visited},
	})
	ts.Require().NoError(err)

	rows, err := ts.repo.Fetch(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Require().Len(rows, 2)
	ts.WithinDuration(visited, rows[0].LastVisitedAt, time.Millisecond)
	ts.WithinDuration(ts.baseTime(), rows[1].LastVisitedAt, time.Millisecond)
}

// Test_UpdateLastVisitedWhenNoRows - если ни одна пара пакета не имеет строки статистики,
// возвращается ErrEventStorageRecordsNotAffected (признак деградации, решение за вызывающим,
// см. auth.UserStatistic.Execute) и ничего не создаётся.
func (ts *UserActivityStatPostgresTestSuite) Test_UpdateLastVisitedWhenNoRows() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/UserActivityStat/UpdateLastVisitedWhenNoRows")

	userID := uuid.MustParse(fixtureUserA)

	err := ts.repo.UpdateLastVisited(ts.ctx, []dto.UserActivityLastVisited{
		{UserID: userID, RealmID: realmA, LastVisitedAt: ts.baseTime()},
	})
	ts.Require().ErrorIs(err, errors.ErrEventStorageRecordsNotAffected)

	rows, err := ts.repo.Fetch(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Empty(rows)
}

// Test_UpdateLastVisitedWhenEmpty - пустой пакет не доходит до БД и не ошибка.
func (ts *UserActivityStatPostgresTestSuite) Test_UpdateLastVisitedWhenEmpty() {
	ts.Require().NoError(ts.repo.UpdateLastVisited(ts.ctx, nil))
}
