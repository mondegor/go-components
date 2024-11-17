package repository_test

import (
	"context"
	"testing"

	"github.com/mondegor/go-storage/mrsql"
	"github.com/mondegor/go-storage/mrtests/infra"
	"github.com/mondegor/go-webcore/mrtests/helpers"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrmailer/dto"
	"github.com/mondegor/go-components/mrmailer/entity"
	"github.com/mondegor/go-components/mrmailer/repository"
	"github.com/mondegor/go-components/tests"
)

type RepositoryTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *infra.PostgresTester
	repo *repository.MessagePostgres
}

func TestMessagePostgresTestSuite(t *testing.T) {
	suite.Run(t, new(RepositoryTestSuite))
}

func (ts *RepositoryTestSuite) SetupSuite() {
	ts.ctx = helpers.ContextWithNopLogger()
	ts.pgt = infra.NewPostgresTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(tests.AppWorkDir() + "/mrmailer/sample/migrations")

	ts.repo = repository.NewMessagePostgres(
		ts.pgt.ConnManager(),
		mrsql.DBTableInfo{
			Name:       "sample_schema.mrmailer_messages",
			PrimaryKey: "message_id",
		},
	)
}

func (ts *RepositoryTestSuite) TearDownSuite() {
	ts.pgt.Destroy(ts.ctx)
}

func (ts *RepositoryTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.ctx)
}

func (ts *RepositoryTestSuite) Test_Fetch() {
	ts.pgt.ApplyFixtures("testdata/Fetch")

	expected := entity.Message{
		ID:      2,
		Channel: "mail",
		Data: dto.MessageData{
			Header: map[string]string{
				"CorrelationID": "56a8ee4a-7fcf-44c5-849e-e9f6a453e380",
			},
			Email: &dto.DataEmail{
				ContentType: "text/plain",
				From: dto.EmailAddress{
					Name:  "Ivan Ivanov",
					Email: "ivan.ivanov@localhost",
				},
				To: dto.EmailAddress{
					Name:  "Ivan Ivanov",
					Email: "ivan.ivanov@localhost",
				},
				ReplyTo: &dto.EmailAddress{
					Name:  "Ivan Ivanov",
					Email: "reply@localhost",
				},
				Subject: "Test Subject",
				Content: "Test Content",
			},
		},
	}

	ctx := context.Background()
	got, err := ts.repo.FetchByIDs(ctx, []uint64{expected.ID})

	ts.Require().NoError(err)
	ts.Equal(expected, got[0])
}

func (ts *RepositoryTestSuite) Test_Insert() {
	ts.pgt.ApplyFixtures("testdata/Insert")

	expected := entity.Message{
		ID:      2,
		Channel: "mail",
		Data: dto.MessageData{
			Header: map[string]string{
				"CorrelationID": "56a8ee4a-7fcf-44c5-849e-e9f6a453e380",
			},
			Email: &dto.DataEmail{
				ContentType: "text/plain",
				From: dto.EmailAddress{
					Name:  "Ivan Ivanov",
					Email: "ivan.ivanov@localhost",
				},
				To: dto.EmailAddress{
					Name:  "Ivan Ivanov",
					Email: "ivan.ivanov@localhost",
				},
				ReplyTo: &dto.EmailAddress{
					Name:  "Ivan Ivanov",
					Email: "reply@localhost",
				},
				Subject: "Test Subject",
				Content: "Test Content",
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
