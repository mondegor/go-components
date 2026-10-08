package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage/mrsql"
	"github.com/mondegor/go-storage/mrtests/pgtest"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrqueue/repository"
	"github.com/mondegor/go-components/tests"
)

const completedTableName = "sample_schema.mrqueue_completed"

type CompletedPostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *pgtest.Tester
	repo *repository.CompletedPostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestCompletedPostgresTestSuite(t *testing.T) {
	suite.Run(t, new(CompletedPostgresTestSuite))
}

func (ts *CompletedPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrqueue"))

	ts.repo = repository.NewCompletedPostgres(
		ts.pgt.ConnManager(),
		mrsql.DBTableInfo{
			Name:       completedTableName,
			PrimaryKey: "item_id",
		},
	)
}

func (ts *CompletedPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// Test_Insert - запись добавляется в список успешно обработанных с текущим временем.
func (ts *CompletedPostgresTestSuite) Test_Insert() {
	ts.Require().NoError(ts.repo.Insert(ts.ctx, 1))

	var updatedAt time.Time

	err := ts.pgt.ConnManager().Conn(ts.ctx).QueryRow(
		ts.ctx,
		`SELECT updated_at FROM `+completedTableName+` WHERE item_id = $1;`,
		1,
	).Scan(&updatedAt)
	ts.Require().NoError(err)
	ts.WithinDuration(time.Now(), updatedAt, time.Minute)
}

// Test_InsertWhenAlreadyCompleted - повторное добавление той же записи отвергается
// нарушением уникальности и не дублирует строку.
func (ts *CompletedPostgresTestSuite) Test_InsertWhenAlreadyCompleted() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Completed/InsertWhenAlreadyCompleted")

	err := ts.repo.Insert(ts.ctx, 1)
	ts.Require().ErrorIs(err, errors.ErrInternalStorageDuplicateKeyViolation)
	ts.Equal(1, ts.pgt.CountRows(ts.T(), ts.ctx, completedTableName))
}

// Test_Delete - записи старше expiry удаляются не более limit за вызов, начиная с самых давних;
// возвращаются их ID.
func (ts *CompletedPostgresTestSuite) Test_Delete() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Completed/Delete")

	ids, err := ts.repo.Delete(ts.ctx, time.Hour, 1)
	ts.Require().NoError(err)
	ts.Equal([]uint64{1}, ids)

	ids, err = ts.repo.Delete(ts.ctx, time.Hour, 10)
	ts.Require().NoError(err)
	ts.Equal([]uint64{2}, ids)

	ids, err = ts.repo.Delete(ts.ctx, time.Hour, 10)
	ts.Require().NoError(err)
	ts.Empty(ids)
	ts.Equal(1, ts.pgt.CountRows(ts.T(), ts.ctx, completedTableName))
}
