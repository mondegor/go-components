package repository_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrpostgres/builder/part"
	"github.com/mondegor/go-core/mrstorage/mrsql"
	"github.com/mondegor/go-storage/mrtests/pgtest"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrsettings/entity"
	"github.com/mondegor/go-components/mrsettings/enum/settingtype"
	"github.com/mondegor/go-components/mrsettings/repository"
	"github.com/mondegor/go-components/tests"
)

type SettingPostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *pgtest.Tester
	repo *repository.SettingPostgres

	// repoWithCondition - репозиторий с условием хоста, которое скрывает настройки с setting_id >= 100.
	repoWithCondition *repository.SettingPostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestSettingPostgresTestSuite(t *testing.T) {
	suite.Run(t, new(SettingPostgresTestSuite))
}

func (ts *SettingPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrsettings"))

	table := mrsql.DBTableInfo{
		Name:       settingsTableName,
		PrimaryKey: "setting_id",
	}

	ts.repo = repository.NewSettingPostgres(ts.pgt.ConnManager(), table, part.NewSQLConditionBuilder(), nil)
	ts.repoWithCondition = repository.NewSettingPostgres(
		ts.pgt.ConnManager(),
		table,
		part.NewSQLConditionBuilder(),
		func(argumentNumber int) (string, []any) {
			return "setting_id < $" + strconv.Itoa(argumentNumber), []any{100}
		},
	)
}

func (ts *SettingPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// Test_Fetch - без даты последнего обновления возвращаются все настройки со всеми полями.
func (ts *SettingPostgresTestSuite) Test_Fetch() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Setting/Fetch")

	rows, err := ts.repo.Fetch(ts.ctx, time.Time{})
	ts.Require().NoError(err)
	ts.ElementsMatch(
		[]entity.Setting{
			{
				ID:          1,
				Name:        "SampleString",
				Type:        settingtype.String,
				Value:       "old",
				Description: "Sample string setting",
				UpdatedAt:   time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC),
			},
			{
				ID:          2,
				Name:        "SampleInteger",
				Type:        settingtype.Integer,
				Value:       "10",
				Description: "Sample integer setting",
				UpdatedAt:   time.Date(2026, 6, 14, 13, 0, 0, 0, time.UTC),
			},
			{
				ID:          100,
				Name:        "HiddenBoolean",
				Type:        settingtype.Boolean,
				Value:       "true",
				Description: "Hidden boolean setting",
				UpdatedAt:   time.Date(2026, 6, 14, 14, 0, 0, 0, time.UTC),
			},
		},
		rows,
	)
}

// Test_FetchWhenLastUpdated - возвращаются только настройки, обновлённые строго позже указанной даты.
func (ts *SettingPostgresTestSuite) Test_FetchWhenLastUpdated() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Setting/Fetch")

	rows, err := ts.repo.Fetch(ts.ctx, time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC))
	ts.Require().NoError(err)
	ts.ElementsMatch([]uint64{2, 100}, ts.settingIDs(rows))
}

// Test_FetchWhenCondition - настройки, не прошедшие условие хоста, не возвращаются.
func (ts *SettingPostgresTestSuite) Test_FetchWhenCondition() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Setting/Fetch")

	rows, err := ts.repoWithCondition.Fetch(ts.ctx, time.Time{})
	ts.Require().NoError(err)
	ts.ElementsMatch([]uint64{1, 2}, ts.settingIDs(rows))
}

// Test_FetchOne - настройка читается по ID: имя, тип и значение.
func (ts *SettingPostgresTestSuite) Test_FetchOne() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Setting/FetchOne")

	row, err := ts.repoWithCondition.FetchOne(ts.ctx, 2)
	ts.Require().NoError(err)
	ts.Equal(entity.Setting{ID: 2, Name: "SampleInteger", Type: settingtype.Integer, Value: "10"}, row)
}

// Test_FetchOneWhenNotExists - неизвестная настройка возвращает ErrEventStorageNoRecordFound.
func (ts *SettingPostgresTestSuite) Test_FetchOneWhenNotExists() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Setting/FetchOne")

	_, err := ts.repo.FetchOne(ts.ctx, 99)
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_FetchOneWhenCondition - настройка, не прошедшая условие хоста, не находится:
// ErrEventStorageNoRecordFound.
func (ts *SettingPostgresTestSuite) Test_FetchOneWhenCondition() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Setting/FetchOne")

	_, err := ts.repoWithCondition.FetchOne(ts.ctx, 100)
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_Update - значение настройки обновляется и сдвигается время её обновления.
func (ts *SettingPostgresTestSuite) Test_Update() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Setting/Update")

	err := ts.repoWithCondition.Update(ts.ctx, entity.Setting{ID: 1, Type: settingtype.String, Value: "new"})
	ts.Require().NoError(err)

	rows, err := ts.repo.Fetch(ts.ctx, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC))
	ts.Require().NoError(err)
	ts.Require().Len(rows, 1)
	ts.Equal(uint64(1), rows[0].ID)
	ts.Equal("new", rows[0].Value)
	ts.WithinDuration(time.Now(), rows[0].UpdatedAt, time.Minute)
}

// Test_UpdateWhenTypeMismatch - настройка не обновляется значением другого типа:
// возвращается ErrEventStorageNoRecordFound.
func (ts *SettingPostgresTestSuite) Test_UpdateWhenTypeMismatch() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Setting/Update")

	err := ts.repo.Update(ts.ctx, entity.Setting{ID: 1, Type: settingtype.Integer, Value: "5"})
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
	ts.Equal("old", ts.settingValue(1))
}

// Test_UpdateWhenNotExists - обновление неизвестной настройки возвращает ErrEventStorageNoRecordFound.
func (ts *SettingPostgresTestSuite) Test_UpdateWhenNotExists() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Setting/Update")

	err := ts.repo.Update(ts.ctx, entity.Setting{ID: 99, Type: settingtype.String, Value: "new"})
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_UpdateWhenCondition - настройка, не прошедшая условие хоста, не обновляется:
// возвращается ErrEventStorageNoRecordFound.
func (ts *SettingPostgresTestSuite) Test_UpdateWhenCondition() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Setting/Update")

	err := ts.repoWithCondition.Update(ts.ctx, entity.Setting{ID: 100, Type: settingtype.Boolean, Value: "false"})
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
	ts.Equal("true", ts.settingValue(100))
}

// settingIDs - идентификаторы указанных настроек.
func (ts *SettingPostgresTestSuite) settingIDs(rows []entity.Setting) []uint64 {
	ids := make([]uint64, 0, len(rows))

	for _, row := range rows {
		ids = append(ids, row.ID)
	}

	return ids
}

// settingValue - текущее значение настройки (через репозиторий без условия хоста).
func (ts *SettingPostgresTestSuite) settingValue(id uint64) string {
	row, err := ts.repo.FetchOne(ts.ctx, id)
	ts.Require().NoError(err)

	return row.Value
}
