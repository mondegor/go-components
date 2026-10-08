package repository_test

import (
	"context"
	"testing"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage/mrsql"
	"github.com/mondegor/go-storage/mrtests/pgtest"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrnotifier/notifier/entity"
	"github.com/mondegor/go-components/mrnotifier/notifier/repository"
	"github.com/mondegor/go-components/tests"
)

const noticesTableName = "sample_schema.notifier_notices"

type NotePostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *pgtest.Tester
	repo *repository.NotePostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestNotePostgresTestSuite(t *testing.T) {
	suite.Run(t, new(NotePostgresTestSuite))
}

func (ts *NotePostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrnotifier"))

	ts.repo = repository.NewNotePostgres(
		ts.pgt.ConnManager(),
		mrsql.DBTableInfo{
			Name:       noticesTableName,
			PrimaryKey: "notice_id",
		},
	)
}

func (ts *NotePostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// Test_FetchByIDs - возвращаются только уведомления с указанными ID, данные читаются из jsonb без искажений.
func (ts *NotePostgresTestSuite) Test_FetchByIDs() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Note/FetchByIDs")

	rows, err := ts.repo.FetchByIDs(ts.ctx, []uint64{1, 3, 99})
	ts.Require().NoError(err)
	ts.ElementsMatch(
		[]entity.Note{
			{ID: 1, Key: "user.welcome", Data: map[string]string{"user_name": "Ivan"}},
			{ID: 3, Key: "user.welcome", Data: map[string]string{"user_name": "Petr", "site_name": "Example"}},
		},
		rows,
	)
}

// Test_FetchByIDsWhenNotExists - по неизвестным ID возвращается пустой срез, а не ошибка.
func (ts *NotePostgresTestSuite) Test_FetchByIDsWhenNotExists() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Note/FetchByIDs")

	rows, err := ts.repo.FetchByIDs(ts.ctx, []uint64{99})
	ts.Require().NoError(err)
	ts.Empty(rows)
}

// Test_Insert - уведомление сохраняется и читается без искажений.
func (ts *NotePostgresTestSuite) Test_Insert() {
	note := entity.Note{ID: 1, Key: "user.welcome", Data: map[string]string{"user_name": "Ivan"}}

	ts.Require().NoError(ts.repo.Insert(ts.ctx, note))

	rows, err := ts.repo.FetchByIDs(ts.ctx, []uint64{1})
	ts.Require().NoError(err)
	ts.Equal([]entity.Note{note}, rows)
}

// Test_InsertWhenAlreadyExists - уведомление с занятым ID отвергается нарушением уникальности.
func (ts *NotePostgresTestSuite) Test_InsertWhenAlreadyExists() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Note/InsertWhenAlreadyExists")

	err := ts.repo.Insert(ts.ctx, entity.Note{ID: 1, Key: "user.farewell", Data: map[string]string{}})
	ts.Require().ErrorIs(err, errors.ErrInternalStorageDuplicateKeyViolation)
	ts.Equal(1, ts.pgt.CountRows(ts.T(), ts.ctx, noticesTableName))
}

// Test_DeleteByIDs - удаляются только уведомления с указанными ID.
func (ts *NotePostgresTestSuite) Test_DeleteByIDs() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Note/DeleteByIDs")

	ts.Require().NoError(ts.repo.DeleteByIDs(ts.ctx, []uint64{1, 3, 99}))

	rows, err := ts.repo.FetchByIDs(ts.ctx, []uint64{1, 2, 3})
	ts.Require().NoError(err)
	ts.Require().Len(rows, 1)
	ts.Equal(uint64(2), rows[0].ID)
}

// Test_DeleteByIDsWhenNotExists - удаление неизвестных ID не ошибка (удаление идемпотентно).
func (ts *NotePostgresTestSuite) Test_DeleteByIDsWhenNotExists() {
	ts.Require().NoError(ts.repo.DeleteByIDs(ts.ctx, []uint64{99}))
}
