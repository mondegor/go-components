package repository_test

import (
	"context"
	"testing"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage/mrsql"
	"github.com/mondegor/go-storage/mrtests/pgtest"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrsettings/repository"
	"github.com/mondegor/go-components/tests"
)

const (
	settingsTableName    = "sample_schema.sample_settings"
	settingsLogTableName = "sample_schema.sample_settings_log"
)

type SettingLogPostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *pgtest.Tester
	repo *repository.SettingLogPostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestSettingLogPostgresTestSuite(t *testing.T) {
	suite.Run(t, new(SettingLogPostgresTestSuite))
}

func (ts *SettingLogPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrsettings"))

	ts.repo = repository.NewSettingLogPostgres(
		ts.pgt.ConnManager(),
		settingsLogTableName,
		mrsql.DBTableInfo{
			Name:       settingsTableName,
			PrimaryKey: "setting_id",
		},
	)
}

func (ts *SettingLogPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// Test_Insert - в журнал попадают имя настройки, её текущее значение как старое и новое значение;
// другие настройки не журналируются.
func (ts *SettingLogPostgresTestSuite) Test_Insert() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/SettingLog/Insert")

	ts.Require().NoError(ts.repo.Insert(ts.ctx, 1, "new"))

	var name, newValue, oldValue string

	err := ts.pgt.ConnManager().Conn(ts.ctx).QueryRow(
		ts.ctx,
		`SELECT setting_name, setting_new_value, setting_old_value FROM `+settingsLogTableName+` WHERE setting_id = $1;`,
		1,
	).Scan(&name, &newValue, &oldValue)
	ts.Require().NoError(err)
	ts.Equal("SampleString", name)
	ts.Equal("new", newValue)
	ts.Equal("old", oldValue)

	ts.Equal(1, ts.pgt.CountRows(ts.T(), ts.ctx, settingsLogTableName))
}

// Test_InsertWhenSettingNotExists - для неизвестной настройки запись не создаётся
// и возвращается ErrEventStorageNoRecordFound.
func (ts *SettingLogPostgresTestSuite) Test_InsertWhenSettingNotExists() {
	err := ts.repo.Insert(ts.ctx, 99, "new")
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
	ts.Equal(0, ts.pgt.CountRows(ts.T(), ts.ctx, settingsLogTableName))
}
