package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/mondegor/go-core/mrstorage/mrsql"
	"github.com/mondegor/go-storage/mrtests/pgtest"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrqueue/entity"
	"github.com/mondegor/go-components/mrqueue/repository"
	"github.com/mondegor/go-components/tests"
)

const crashedTableName = "sample_schema.mrqueue_errors"

type CrashedPostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *pgtest.Tester
	repo *repository.CrashedPostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestCrashedPostgresTestSuite(t *testing.T) {
	suite.Run(t, new(CrashedPostgresTestSuite))
}

func (ts *CrashedPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrqueue"))

	ts.repo = repository.NewCrashedPostgres(
		ts.pgt.ConnManager(),
		mrsql.DBTableInfo{
			Name:       crashedTableName,
			PrimaryKey: "item_id",
		},
	)
}

func (ts *CrashedPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// Test_Insert - в журнал добавляется по строке на каждую ошибку, в том числе несколько
// ошибок одной записи.
func (ts *CrashedPostgresTestSuite) Test_Insert() {
	err := ts.repo.Insert(ts.ctx, []entity.CrashedItem{
		{ID: 1, Cause: "first error"},
		{ID: 1, Cause: "second error"},
		{ID: 2, Cause: "error"},
	})
	ts.Require().NoError(err)

	ts.Equal(
		[]entity.CrashedItem{
			{ID: 1, Cause: "first error"},
			{ID: 1, Cause: "second error"},
			{ID: 2, Cause: "error"},
		},
		ts.fetchItems(),
	)
}

// Test_InsertWhenEmpty - пустой список не доходит до БД и не ошибка.
func (ts *CrashedPostgresTestSuite) Test_InsertWhenEmpty() {
	ts.Require().NoError(ts.repo.Insert(ts.ctx, nil))
}

// Test_InsertOne - в журнал добавляется одна ошибка.
func (ts *CrashedPostgresTestSuite) Test_InsertOne() {
	ts.Require().NoError(ts.repo.InsertOne(ts.ctx, entity.CrashedItem{ID: 1, Cause: "error"}))
	ts.Equal([]entity.CrashedItem{{ID: 1, Cause: "error"}}, ts.fetchItems())
}

// Test_Delete - журнал записи удаляется целиком, когда её последняя ошибка старше expiry;
// за вызов - не более limit записей, начиная с самых давних; возвращаются их ID.
func (ts *CrashedPostgresTestSuite) Test_Delete() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Crashed/Delete")

	ids, err := ts.repo.Delete(ts.ctx, time.Hour, 1)
	ts.Require().NoError(err)
	ts.Equal([]uint64{1}, ids)

	ids, err = ts.repo.Delete(ts.ctx, time.Hour, 10)
	ts.Require().NoError(err)
	ts.Equal([]uint64{3}, ids)

	ids, err = ts.repo.Delete(ts.ctx, time.Hour, 10)
	ts.Require().NoError(err)
	ts.Empty(ids)

	ts.Equal(
		[]entity.CrashedItem{{ID: 2, Cause: "old error"}, {ID: 2, Cause: "new error"}},
		ts.fetchItems(),
	)
}

// fetchItems - все строки журнала в порядке item_id и времени ошибки.
func (ts *CrashedPostgresTestSuite) fetchItems() []entity.CrashedItem {
	rows, err := ts.pgt.ConnManager().Conn(ts.ctx).Query(
		ts.ctx,
		`SELECT item_id, error_message FROM `+crashedTableName+` ORDER BY item_id, created_at, error_message;`,
	)
	ts.Require().NoError(err)

	defer rows.Close()

	var items []entity.CrashedItem

	for rows.Next() {
		var item entity.CrashedItem

		ts.Require().NoError(rows.Scan(&item.ID, &item.Cause))

		items = append(items, item)
	}

	ts.Require().NoError(rows.Err())

	return items
}
