package repository_test

import (
	"context"
	"testing"

	"github.com/mondegor/go-core/mrstorage/mrsql"
	"github.com/mondegor/go-storage/mrtests/pgtest"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrmailer"
	"github.com/mondegor/go-components/mrmailer/entity"
	"github.com/mondegor/go-components/mrmailer/repository"
	"github.com/mondegor/go-components/tests"
)

type MessagePostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *pgtest.Tester
	repo *repository.MessagePostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestMessagePostgresTestSuite(t *testing.T) {
	suite.Run(t, new(MessagePostgresTestSuite))
}

func (ts *MessagePostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrmailer"))

	ts.repo = repository.NewMessagePostgres(
		ts.pgt.ConnManager(),
		mrsql.DBTableInfo{
			Name:       "sample_schema.mrmailer_messages",
			PrimaryKey: "message_id",
		},
	)
}

func (ts *MessagePostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// Test_FetchByIDs - сообщение читается по ID без искажений, включая данные из jsonb-колонки.
func (ts *MessagePostgresTestSuite) Test_FetchByIDs() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Message/FetchByIDs")

	expected := entity.Message{
		ID:      2,
		Channel: "mail",
		Data: entity.MessageData{
			Header: map[string]string{
				mrmailer.HeaderCorrelationID: "56a8ee4a-7fcf-44c5-849e-e9f6a453e380",
			},
			Mail: &entity.DataMail{
				ContentType: "text/plain",
				From:        "Ivan Ivanov",
				To:          "Ivan Ivanov <ivan.ivanov@localhost>",
				ReplyTo:     "Ivan Ivanov <reply@localhost>",
				Subject:     "Test Subject",
				Content:     "Test Content",
			},
		},
	}

	ctx := context.Background()
	got, err := ts.repo.FetchByIDs(ctx, []uint64{expected.ID})

	ts.Require().NoError(err)
	ts.Equal(expected, got[0])
}

func (ts *MessagePostgresTestSuite) Test_Insert() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Message/Insert")

	expected := entity.Message{
		ID:      2,
		Channel: "mail",
		Data: entity.MessageData{
			Header: map[string]string{
				mrmailer.HeaderCorrelationID: "56a8ee4a-7fcf-44c5-849e-e9f6a453e380",
			},
			Mail: &entity.DataMail{
				ContentType: "text/plain",
				From:        "Ivan Ivanov",
				To:          "Ivan Ivanov <ivan.ivanov@localhost>",
				ReplyTo:     "Ivan Ivanov <reply@localhost>",
				Subject:     "Test Subject",
				Content:     "Test Content",
			},
		},
	}

	ctx := context.Background()
	err := ts.repo.Insert(ctx, []entity.Message{expected})

	ts.Require().NoError(err)

	got, err := ts.repo.FetchByIDs(ctx, []uint64{expected.ID})

	ts.Require().NoError(err)
	ts.Equal(expected, got[0])
}

// Test_DeleteByIDs - удаляются сообщения только с указанными ID, неизвестные ID и пустой
// список не ошибка (удаление идемпотентно).
func (ts *MessagePostgresTestSuite) Test_DeleteByIDs() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Message/DeleteByIDs")

	ts.Require().NoError(ts.repo.DeleteByIDs(ts.ctx, []uint64{1, 3, 99}))

	got, err := ts.repo.FetchByIDs(ts.ctx, []uint64{1, 2, 3})
	ts.Require().NoError(err)
	ts.Require().Len(got, 1)
	ts.Equal(uint64(2), got[0].ID)

	ts.Require().NoError(ts.repo.DeleteByIDs(ts.ctx, []uint64{1, 3}))
	ts.Require().NoError(ts.repo.DeleteByIDs(ts.ctx, nil))
	ts.Equal(1, ts.pgt.CountRows(ts.T(), ts.ctx, "sample_schema.mrmailer_messages"))
}
