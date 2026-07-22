package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-storage/mrtests/infra"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/tests"
)

const usersTableName = "sample_schema.users"

type UserPostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *infra.PostgresTester
	repo *repository.UserPostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestUserPostgresTestSuite(t *testing.T) {
	suite.Run(t, new(UserPostgresTestSuite))
}

func (ts *UserPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = infra.NewPostgresTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(tests.AppWorkDir() + "/mrauth/_sample/migrations")
	ts.repo = repository.NewUserPostgres(ts.pgt.ConnManager(), usersTableName)
}

func (ts *UserPostgresTestSuite) TearDownSuite() {
	ts.pgt.Destroy(ts.ctx)
}

func (ts *UserPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.ctx)
}

// seededAt - опорный момент updated_at засеянного пользователя (заведомо в прошлом),
// чтобы наблюдать его сдвиг после обновления (updated_at = NOW()).
func (ts *UserPostgresTestSuite) seededAt() time.Time {
	return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
}

// seedUser - вставляет пользователя с заданными языком и часовым поясом и возвращает его user_id.
// updated_at засевается в прошлом, чтобы тест мог зафиксировать его обновление.
func (ts *UserPostgresTestSuite) seedUser(langCode, timeZone string) uuid.UUID {
	userID := uuid.New()

	sql := `
		INSERT INTO sample_schema.users
			(user_id, user_email, lang_code, user_timezone, registered_ip, user_status, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7);`

	err := ts.pgt.ConnManager().Conn(ts.ctx).Exec(
		ts.ctx,
		sql,
		userID,
		userID.String()+"@localhost",
		langCode,
		timeZone,
		"203.0.113.7",
		2, // ENABLED
		ts.seededAt(),
	)
	ts.Require().NoError(err)

	return userID
}

// TestUpdateSettings - обновляет язык и часовой пояс одним запросом и сдвигает updated_at.
func (ts *UserPostgresTestSuite) TestUpdateSettings() {
	userID := ts.seedUser("ru-RU", "Europe/Moscow")

	err := ts.repo.UpdateSettings(ts.ctx, entity.UserSettings{
		UserID:   userID,
		LangCode: "en-US",
		TimeZone: "UTC",
	})
	ts.Require().NoError(err)

	row, err := ts.repo.FetchOne(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Equal("en-US", row.LangCode)
	ts.Equal("UTC", row.TimeZone)
	ts.True(row.UpdatedAt.After(ts.seededAt()), "updated_at должен быть обновлён на NOW()")
}

// TestUpdateSettingsUserNotExists - обновление несуществующего пользователя не затрагивает строк
// и возвращает ErrEventStorageNoRecordFound (0 строк в ExecRow), ничего не создавая.
func (ts *UserPostgresTestSuite) TestUpdateSettingsUserNotExists() {
	err := ts.repo.UpdateSettings(ts.ctx, entity.UserSettings{
		UserID:   uuid.New(),
		LangCode: "en-US",
		TimeZone: "UTC",
	})
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// TestUpdateSettingsSoftDeletedUser - мягко удалённый пользователь не обновляется
// (условие deleted_at IS NULL): возвращается ErrEventStorageNoRecordFound.
func (ts *UserPostgresTestSuite) TestUpdateSettingsSoftDeletedUser() {
	userID := ts.seedUser("ru-RU", "Europe/Moscow")

	err := ts.pgt.ConnManager().Conn(ts.ctx).Exec(
		ts.ctx,
		`UPDATE sample_schema.users SET deleted_at = NOW() WHERE user_id = $1;`,
		userID,
	)
	ts.Require().NoError(err)

	err = ts.repo.UpdateSettings(ts.ctx, entity.UserSettings{
		UserID:   userID,
		LangCode: "en-US",
		TimeZone: "UTC",
	})
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}
