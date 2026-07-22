package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	sysmesserrors "github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-storage/mrtests/infra"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/tests"
)

const secureOperationsTableName = "sample_schema.secure_operations"

type SecureOperationPostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *infra.PostgresTester
	repo *repository.SecureOperationPostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestSecureOperationPostgresTestSuite(t *testing.T) {
	suite.Run(t, new(SecureOperationPostgresTestSuite))
}

func (ts *SecureOperationPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = infra.NewPostgresTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(tests.AppWorkDir() + "/mrauth/_sample/migrations")
	ts.repo = repository.NewSecureOperationPostgres(ts.pgt.ConnManager(), secureOperationsTableName)
}

func (ts *SecureOperationPostgresTestSuite) TearDownSuite() {
	ts.pgt.Destroy(ts.ctx)
}

func (ts *SecureOperationPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.ctx)
}

// seedOperation - сохраняет операцию указанного типа для указанного владельца.
func (ts *SecureOperationPostgresTestSuite) seedOperation(userID uuid.UUID, name string) string {
	token := "token-" + uuid.NewString()

	op, err := secureoperation.NewOperation(
		token,
		name,
		userID,
		[]secureoperation.ConfirmAction{
			{
				Method:      confirmmethod.Email,
				MaxAttempts: 3,
				MaxResends:  5,
				Expiry:      10 * time.Minute,
				Address:     "u@e",
				ConfirmCode: "hash",
			},
		},
		nil,
	)
	ts.Require().NoError(err)
	ts.Require().NoError(ts.repo.Insert(ts.ctx, op))

	return token
}

func (ts *SecureOperationPostgresTestSuite) TestDeleteByUserIDAndName() {
	userID := uuid.New()
	otherUserID := uuid.New()

	// две операции одного типа одного пользователя: обе подлежат вытеснению
	ts.seedOperation(userID, "confirm.disable.2fa")
	ts.seedOperation(userID, "confirm.disable.2fa")

	// операция другого типа того же пользователя и операция другого пользователя - не трогаются
	otherNameToken := ts.seedOperation(userID, "confirm.change.email")
	otherUserToken := ts.seedOperation(otherUserID, "confirm.disable.2fa")

	ts.Require().NoError(ts.repo.DeleteByUserIDAndName(ts.ctx, userID, "confirm.disable.2fa"))

	_, err := ts.repo.FetchOne(ts.ctx, otherNameToken)
	ts.Require().NoError(err, "операция другого типа того же пользователя остаётся")

	_, err = ts.repo.FetchOne(ts.ctx, otherUserToken)
	ts.Require().NoError(err, "операция того же типа другого пользователя остаётся")

	// вытеснять больше нечего: на этом построена ветка "первая операция такого типа"
	err = ts.repo.DeleteByUserIDAndName(ts.ctx, userID, "confirm.disable.2fa")
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageRecordsNotAffected)
}
