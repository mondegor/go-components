package repository_test

import (
	"context"
	"testing"

	"github.com/mondegor/go-storage/mrtests/pgtest"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrnotifier/template/entity"
	"github.com/mondegor/go-components/mrnotifier/template/repository"
	"github.com/mondegor/go-components/tests"
)

type VariablePostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *pgtest.Tester
	repo *repository.VariablePostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestVariablePostgresTestSuite(t *testing.T) {
	suite.Run(t, new(VariablePostgresTestSuite))
}

func (ts *VariablePostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrnotifier"))

	ts.repo = repository.NewVariablePostgres(ts.pgt.ConnManager(), "sample_schema.notifier_template_vars")
}

func (ts *VariablePostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// Test_Fetch - возвращаются значения по умолчанию только запрошенных переменных; неизвестные пропускаются.
func (ts *VariablePostgresTestSuite) Test_Fetch() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Variable/Fetch")

	rows, err := ts.repo.Fetch(ts.ctx, []string{"user_name", "site_name", "unknown"})
	ts.Require().NoError(err)
	ts.ElementsMatch(
		[]entity.Variable{
			{Name: "user_name", DefaultValue: "Guest"},
			{Name: "site_name", DefaultValue: "Example"},
		},
		rows,
	)
}

// Test_FetchWhenEmpty - по пустому списку переменных возвращается пустой срез, а не ошибка.
func (ts *VariablePostgresTestSuite) Test_FetchWhenEmpty() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Variable/Fetch")

	rows, err := ts.repo.Fetch(ts.ctx, nil)
	ts.Require().NoError(err)
	ts.Empty(rows)
}
