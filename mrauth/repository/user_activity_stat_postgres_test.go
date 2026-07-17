package repository_test

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-storage/mrtests/infra"
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
	pgt  *infra.PostgresTester
	repo *repository.UserActivityStatPostgres
}

func TestUserActivityStatPostgresTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(UserActivityStatPostgresTestSuite))
}

func (ts *UserActivityStatPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = infra.NewPostgresTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(tests.AppWorkDir() + "/mrauth/_sample/migrations")
	ts.repo = repository.NewUserActivityStatPostgres(ts.pgt.ConnManager(), usersActivityStatTableName)
}

func (ts *UserActivityStatPostgresTestSuite) TearDownSuite() {
	ts.pgt.Destroy(ts.ctx)
}

func (ts *UserActivityStatPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.ctx)
}

// baseTime - опорное время тестов без наносекунд: timestamptz хранит микросекунды.
func (ts *UserActivityStatPostgresTestSuite) baseTime() time.Time {
	return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
}

// seedUser - вставляет запись в users и возвращает её user_id.
// TruncateTables вычищает и строки, засеянные миграцией, поэтому родителей FK создаёт сам тест.
func (ts *UserActivityStatPostgresTestSuite) seedUser() uuid.UUID {
	userID := uuid.New()

	sql := `
		INSERT INTO sample_schema.users
			(user_id, user_email, lang_code, registered_ip, user_status)
		VALUES
			($1, $2, $3, $4, $5);`

	err := ts.pgt.ConnManager().Conn(ts.ctx).Exec(
		ts.ctx,
		sql,
		userID,
		userID.String()+"@localhost",
		"ru-RU",
		"203.0.113.7",
		2, // ENABLED
	)
	ts.Require().NoError(err)

	return userID
}

// seedUserRealm - привязывает пользователя к realm: без этой строки FK не даст создать статистику.
func (ts *UserActivityStatPostgresTestSuite) seedUserRealm(userID uuid.UUID, realmID uint16) {
	sql := `
		INSERT INTO sample_schema.users_realms
			(user_id, realm_id, user_kind)
		VALUES
			($1, $2, $3);`

	err := ts.pgt.ConnManager().Conn(ts.ctx).Exec(ts.ctx, sql, userID, realmID, "standard")
	ts.Require().NoError(err)
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

// TestFetchOrderedByRealm - статистика пользователя выбирается по всем его realm'ам в порядке realm_id.
func (ts *UserActivityStatPostgresTestSuite) TestFetchOrderedByRealm() {
	userID := ts.seedUser()
	ts.seedUserRealm(userID, realmA)
	ts.seedUserRealm(userID, realmB)

	// realm B вставляется первым, чтобы порядок обеспечивался ORDER BY, а не порядком вставки
	ts.Require().NoError(ts.repo.InsertOrUpdate(ts.ctx, ts.stat(userID, realmB, "198.51.100.9")))
	ts.Require().NoError(ts.repo.InsertOrUpdate(ts.ctx, ts.stat(userID, realmA, "203.0.113.7")))

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

// TestFetchNoRows - у пользователя без статистики Fetch возвращает пустой срез, а не ошибку.
func (ts *UserActivityStatPostgresTestSuite) TestFetchNoRows() {
	userID := ts.seedUser()

	rows, err := ts.repo.Fetch(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Empty(rows)
}

// TestInsertOrUpdateRejectsUnsetLoginIP - незаданный IP входа отвергается ограничением NOT NULL,
// а не записывается как NULL: строка статистики заводится только при входе, поэтому IP входа
// известен всегда (pgx кодирует невалидный netip.Addr как NULL - без ограничения он утёк бы в БД).
// Вызывающий такой сбой не проваливает: запись активности best-effort, см. OpenSession.Execute.
func (ts *UserActivityStatPostgresTestSuite) TestInsertOrUpdateRejectsUnsetLoginIP() {
	userID := ts.seedUser()
	ts.seedUserRealm(userID, realmA)

	row := ts.stat(userID, realmA, "203.0.113.7")
	row.LastLoginIP = netip.Addr{} // IP клиента не распознан

	ts.Require().Error(ts.repo.InsertOrUpdate(ts.ctx, row))

	// строки нет вовсе: частичная запись без IP не создаётся
	rows, err := ts.repo.Fetch(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Empty(rows)
}

// TestInsertOrUpdateOverwrites - повторный вызов для той же пары (user, realm) обновляет строку.
func (ts *UserActivityStatPostgresTestSuite) TestInsertOrUpdateOverwrites() {
	userID := ts.seedUser()
	ts.seedUserRealm(userID, realmA)

	ts.Require().NoError(ts.repo.InsertOrUpdate(ts.ctx, ts.stat(userID, realmA, "203.0.113.7")))

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

// TestInsertOrUpdateRealmsAreIndependent - строки разных realm'ов одного пользователя не затирают друг друга.
func (ts *UserActivityStatPostgresTestSuite) TestInsertOrUpdateRealmsAreIndependent() {
	userID := ts.seedUser()
	ts.seedUserRealm(userID, realmA)
	ts.seedUserRealm(userID, realmB)

	ts.Require().NoError(ts.repo.InsertOrUpdate(ts.ctx, ts.stat(userID, realmA, "203.0.113.7")))
	ts.Require().NoError(ts.repo.InsertOrUpdate(ts.ctx, ts.stat(userID, realmB, "198.51.100.9")))

	rows, err := ts.repo.Fetch(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Require().Len(rows, 2)
	ts.Equal("203.0.113.7", rows[0].LastLoginIP.String())
	ts.Equal("198.51.100.9", rows[1].LastLoginIP.String())
}

// TestUpdateLastVisitedBatch - пакет обновляет last_visited_at строго по паре (user, realm).
func (ts *UserActivityStatPostgresTestSuite) TestUpdateLastVisitedBatch() {
	userID := ts.seedUser()
	ts.seedUserRealm(userID, realmA)
	ts.seedUserRealm(userID, realmB)

	ts.Require().NoError(ts.repo.InsertOrUpdate(ts.ctx, ts.stat(userID, realmA, "203.0.113.7")))
	ts.Require().NoError(ts.repo.InsertOrUpdate(ts.ctx, ts.stat(userID, realmB, "198.51.100.9")))

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

// TestUpdateLastVisitedMissingRow - если ни одна пара пакета не имеет строки статистики,
// возвращается ErrEventStorageRecordsNotAffected (признак деградации, решение за вызывающим,
// см. auth.UserStatistic.Execute) и ничего не создаётся.
func (ts *UserActivityStatPostgresTestSuite) TestUpdateLastVisitedMissingRow() {
	userID := ts.seedUser()
	ts.seedUserRealm(userID, realmA)

	err := ts.repo.UpdateLastVisited(ts.ctx, []dto.UserActivityLastVisited{
		{UserID: userID, RealmID: realmA, LastVisitedAt: ts.baseTime()},
	})
	ts.Require().ErrorIs(err, errors.ErrEventStorageRecordsNotAffected)

	rows, err := ts.repo.Fetch(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Empty(rows)
}

// TestUpdateLastVisitedEmptyBatch - пустой пакет не доходит до БД и не ошибка.
func (ts *UserActivityStatPostgresTestSuite) TestUpdateLastVisitedEmptyBatch() {
	ts.Require().NoError(ts.repo.UpdateLastVisited(ts.ctx, nil))
}
