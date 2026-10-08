package repository_test

import (
	"context"
	"testing"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrworkflow/itemstatus"
	"github.com/mondegor/go-storage/mrtests/pgtest"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrnotifier/template/entity"
	"github.com/mondegor/go-components/mrnotifier/template/repository"
	"github.com/mondegor/go-components/tests"
)

type TemplatePostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *pgtest.Tester
	repo *repository.TemplatePostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestTemplatePostgresTestSuite(t *testing.T) {
	suite.Run(t, new(TemplatePostgresTestSuite))
}

func (ts *TemplatePostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrnotifier"))

	ts.repo = repository.NewTemplatePostgres(ts.pgt.ConnManager(), "sample_schema.notifier_templates")
}

func (ts *TemplatePostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// Test_FetchOneByKey - шаблон выбирается по имени и языку, свойства уведомления
// и список переменных читаются из jsonb без искажений.
func (ts *TemplatePostgresTestSuite) Test_FetchOneByKey() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Template/FetchOneByKey")

	row, err := ts.repo.FetchOneByKey(ts.ctx, "user.welcome", "ru-RU")
	ts.Require().NoError(err)
	ts.Equal(
		entity.Template{
			Name: "user.welcome",
			Lang: "ru-RU",
			Props: entity.TemplateData{
				Mail: &entity.DataMail{Subject: "Привет", Content: "Здравствуйте, {{user_name}}"},
			},
			Vars:   []string{"user_name"},
			Status: itemstatus.Enabled,
		},
		row,
	)

	row, err = ts.repo.FetchOneByKey(ts.ctx, "user.welcome", "en-US")
	ts.Require().NoError(err)
	ts.Equal(
		entity.TemplateData{
			Mail: &entity.DataMail{Subject: "Hello", Content: "Hello, {{user_name}}"},
			SMS:  &entity.DataSMS{Content: "Hi"},
		},
		row.Props,
	)
	ts.Equal(itemstatus.Draft, row.Status)
}

// Test_FetchOneByKeyWhenDeleted - удалённый шаблон не находится: ErrEventStorageNoRecordFound.
func (ts *TemplatePostgresTestSuite) Test_FetchOneByKeyWhenDeleted() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Template/FetchOneByKey")

	_, err := ts.repo.FetchOneByKey(ts.ctx, "user.archived", "ru-RU")
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_FetchOneByKeyWhenNotExists - шаблон с неизвестным именем или без перевода на указанный язык
// не находится: ErrEventStorageNoRecordFound.
func (ts *TemplatePostgresTestSuite) Test_FetchOneByKeyWhenNotExists() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Template/FetchOneByKey")

	_, err := ts.repo.FetchOneByKey(ts.ctx, "user.unknown", "ru-RU")
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)

	_, err = ts.repo.FetchOneByKey(ts.ctx, "user.welcome", "de-DE")
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}
