package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	sysmesserrors "github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-storage/mrtests/infra"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrauth"
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

// TestRecoveryChainRoundTrip - и признак AllowRecovery, и звено RECOVERY живут в jsonb-колонке
// confirm_actions и обязаны переживать запись, чтение и перезапись операции. Признак невидим
// в нулевом значении (omitempty), поэтому проверяется, что снятым он читается как снятый,
// а не как потерянное поле.
//
// Пара "AllowRecovery на звене RECOVERY" здесь синтетическая: ни одна фабрика такую не строит
// (аварийный код вместо аварийного кода), но она единственная, где признак можно выставить
// не нарушив инвариант - поднять его на первое звено checkInvariants не даст.
func (ts *SecureOperationPostgresTestSuite) TestRecoveryChainRoundTrip() {
	token := "token-" + uuid.NewString()

	op, err := secureoperation.NewOperation(
		token,
		"confirm.authorize.user",
		uuid.New(),
		[]secureoperation.ConfirmAction{
			{
				Method:      confirmmethod.TOTP,
				MaxAttempts: 3,
				Expiry:      10 * time.Minute,
			},
			{
				Method:        confirmmethod.Recovery,
				MaxAttempts:   3,
				Expiry:        10 * time.Minute,
				AllowRecovery: true,
			},
		},
		nil,
	)
	ts.Require().NoError(err)
	ts.Require().NoError(ts.repo.Insert(ts.ctx, op))

	stored, err := ts.repo.FetchOne(ts.ctx, token)
	ts.Require().NoError(err)
	ts.Require().Len(stored.Actions(), 2)
	ts.Equal(confirmmethod.TOTP, stored.Actions()[0].Method)
	ts.False(stored.Actions()[0].AllowRecovery)
	ts.Equal(confirmmethod.Recovery, stored.Actions()[1].Method)
	ts.True(stored.Actions()[1].AllowRecovery)

	// первое звено подтверждено - до следующего запроса остаток цепочки доносит Replace
	confirmed, err := stored.ConfirmAction(func(secureoperation.ConfirmAction) (bool, error) {
		return true, nil
	})
	ts.Require().NoError(err)
	ts.Require().False(confirmed)
	ts.Require().NoError(ts.repo.Replace(ts.ctx, token, stored))

	reread, err := ts.repo.FetchOne(ts.ctx, token)
	ts.Require().NoError(err)
	ts.Require().Len(reread.Actions(), 1)
	ts.Equal(confirmmethod.Recovery, reread.Actions()[0].Method)
	ts.True(reread.Actions()[0].AllowRecovery)
}

// TestTwoFactorChainRoundTrip - цепочка "второй фактор -> аварийный код" обязана пережить
// запись, чтение и перезапись операции целиком: оба звена не-sendable, кода подтверждения
// в них нет, и потерянное при сериализации звено превратило бы двухшаговую операцию
// в одношаговую. Такую цепочку строит unit.AuthorizeUserByRecovery.
func (ts *SecureOperationPostgresTestSuite) TestTwoFactorChainRoundTrip() {
	token := "token-" + uuid.NewString()

	op, err := secureoperation.NewOperation(
		token,
		"confirm.authorize.user",
		uuid.New(),
		[]secureoperation.ConfirmAction{
			{
				Method:      confirmmethod.Password,
				MaxAttempts: 3,
				Expiry:      10 * time.Minute,
			},
			{
				Method:      confirmmethod.Recovery,
				MaxAttempts: 3,
				Expiry:      10 * time.Minute,
			},
		},
		nil,
	)
	ts.Require().NoError(err)
	ts.Require().NoError(ts.repo.Insert(ts.ctx, op))

	stored, err := ts.repo.FetchOne(ts.ctx, token)
	ts.Require().NoError(err)
	ts.Require().Len(stored.Actions(), 2)
	ts.Equal(confirmmethod.Password, stored.Actions()[0].Method)
	ts.Equal(confirmmethod.Recovery, stored.Actions()[1].Method)

	// неверное доказательство расходует попытку, и остаток цепочки вместе с ней
	// до следующего запроса доносит Replace
	confirmed, err := stored.ConfirmAction(func(_ secureoperation.ConfirmAction) (bool, error) {
		return false, nil
	})
	ts.Require().ErrorIs(err, mrauth.ErrConfirmCodeIsIncorrect)
	ts.Require().False(confirmed)
	ts.Require().NoError(ts.repo.Replace(ts.ctx, token, stored))

	reread, err := ts.repo.FetchOne(ts.ctx, token)
	ts.Require().NoError(err)
	ts.Require().Len(reread.Actions(), 2)
	ts.Equal(confirmmethod.Password, reread.Actions()[0].Method)
	ts.Equal(int16(2), reread.RemainingAttempts)
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
