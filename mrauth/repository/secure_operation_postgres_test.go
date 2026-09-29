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
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
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
func (ts *SecureOperationPostgresTestSuite) seedOperation(userID uuid.UUID, opType operationtype.Enum) string {
	token := "token-" + uuid.NewString()

	op, err := secureoperation.NewOperation(
		token,
		opType,
		userID,
		[]secureoperation.ConfirmAction{
			{
				Method:      confirmmethod.Email,
				CodeLength:  6,
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
		operationtype.AuthorizeUser,
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

// TestFixedExpiryRoundTrip - фиксированный срок (срок звена больше порога модели),
// назначенный при создании, переживает запись, чтение,
// повторную отправку кода и перезапись операции под новым токеном: срок звена живёт
// в jsonb-колонке confirm_actions, и по нему после чтения решается, продлевать ли срок.
func (ts *SecureOperationPostgresTestSuite) TestFixedExpiryRoundTrip() {
	token := "token-" + uuid.NewString()

	op, err := secureoperation.NewOperation(
		token,
		operationtype.ChangeEmailConfirm,
		uuid.New(),
		[]secureoperation.ConfirmAction{
			{
				Method:      confirmmethod.Email,
				CodeLength:  6,
				MaxAttempts: 3,
				MaxResends:  5,
				Expiry:      72 * time.Hour,
				Address:     "new@example.com",
				ConfirmCode: "hash",
			},
		},
		nil,
	)
	ts.Require().NoError(err)
	ts.Require().NoError(ts.repo.Insert(ts.ctx, op))

	stored, err := ts.repo.FetchOne(ts.ctx, token)
	ts.Require().NoError(err)
	ts.Require().Len(stored.Actions(), 1)
	ts.Equal(72*time.Hour, stored.Actions()[0].Expiry)
	ts.WithinDuration(op.ExpiresAt, stored.ExpiresAt, time.Second)

	// пауза между отправками к сроку операции отношения не имеет - считаем её истёкшей
	stored.ResendsAt = time.Now().UTC().Add(-time.Minute)

	newToken := "token-" + uuid.NewString()
	ts.Require().NoError(stored.ActivateResendCode(newToken))
	ts.Require().NoError(ts.repo.Replace(ts.ctx, token, stored))

	reread, err := ts.repo.FetchOne(ts.ctx, newToken)
	ts.Require().NoError(err)
	ts.WithinDuration(op.ExpiresAt, reread.ExpiresAt, time.Second)
}

// TestTwoFactorChainRoundTrip - цепочка "второй фактор -> аварийный код" обязана пережить
// запись, чтение и перезапись операции целиком: оба звена не-sendable, кода подтверждения
// в них нет, и потерянное при сериализации звено превратило бы двухшаговую операцию
// в одношаговую. Такую цепочку строит unit.AuthorizeUserByRecovery.
func (ts *SecureOperationPostgresTestSuite) TestTwoFactorChainRoundTrip() {
	token := "token-" + uuid.NewString()

	op, err := secureoperation.NewOperation(
		token,
		operationtype.AuthorizeUser,
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

// TestDeleteByUserIDAndTypes - вытесняет все операции указанных типов пользователя, возвращает
// их типы (по одному на операцию) и не трогает ни операции других типов, ни операции тех же
// типов другого пользователя. Когда вытеснять нечего - пустой срез без ошибки.
func (ts *SecureOperationPostgresTestSuite) TestDeleteByUserIDAndTypes() {
	userID := uuid.New()
	otherUserID := uuid.New()
	types := []operationtype.Enum{operationtype.ChangeEmail, operationtype.ChangeEmailConfirm}

	// операции обоих типов цепочки, одна из них в двух экземплярах: все подлежат вытеснению
	firstToken := ts.seedOperation(userID, operationtype.ChangeEmail)
	secondToken := ts.seedOperation(userID, operationtype.ChangeEmail)
	thirdToken := ts.seedOperation(userID, operationtype.ChangeEmailConfirm)

	otherTypeToken := ts.seedOperation(userID, operationtype.Disable2FA)
	otherUserToken := ts.seedOperation(otherUserID, operationtype.ChangeEmailConfirm)

	deleted, err := ts.repo.DeleteByUserIDAndTypes(ts.ctx, userID, types)
	ts.Require().NoError(err)
	ts.ElementsMatch(
		[]operationtype.Enum{operationtype.ChangeEmail, operationtype.ChangeEmail, operationtype.ChangeEmailConfirm},
		deleted,
	)

	for _, token := range []string{firstToken, secondToken, thirdToken} {
		_, err = ts.repo.FetchOne(ts.ctx, token)
		ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)
	}

	_, err = ts.repo.FetchOne(ts.ctx, otherTypeToken)
	ts.Require().NoError(err, "операция другого типа того же пользователя остаётся")

	_, err = ts.repo.FetchOne(ts.ctx, otherUserToken)
	ts.Require().NoError(err, "операция того же типа другого пользователя остаётся")

	deleted, err = ts.repo.DeleteByUserIDAndTypes(ts.ctx, userID, types)
	ts.Require().NoError(err)
	ts.Empty(deleted)
}

// TestDeleteByUserID - удаляет все операции пользователя любого типа и в любом статусе, включая
// подтверждённые, ждущие применения (иначе такую операцию можно было бы применить после отзыва),
// и возвращает их типы (по одному на операцию); операции другого пользователя не трогает.
// У пользователя без операций - пустой срез без ошибки: отзывать нечего - штатный случай.
func (ts *SecureOperationPostgresTestSuite) TestDeleteByUserID() {
	userID := uuid.New()
	otherUserID := uuid.New()

	loginToken := ts.seedOperation(userID, operationtype.AuthorizeUser)
	phoneToken := ts.seedOperation(userID, operationtype.ChangePhone)
	confirmedToken := ts.seedOperation(userID, operationtype.ChangePhone)

	// подтверждённая операция: звенья пройдены, ждёт применения
	confirmed, err := ts.repo.FetchOne(ts.ctx, confirmedToken)
	ts.Require().NoError(err)
	isConfirmed, err := confirmed.ConfirmAction(func(secureoperation.ConfirmAction) (bool, error) { return true, nil })
	ts.Require().NoError(err)
	ts.Require().True(isConfirmed)
	ts.Require().NoError(ts.repo.Replace(ts.ctx, confirmedToken, confirmed))

	otherUserToken := ts.seedOperation(otherUserID, operationtype.ChangePhone)

	types, err := ts.repo.DeleteByUserID(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.ElementsMatch([]operationtype.Enum{operationtype.AuthorizeUser, operationtype.ChangePhone, operationtype.ChangePhone}, types)

	_, err = ts.repo.FetchOne(ts.ctx, loginToken)
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)

	_, err = ts.repo.FetchOne(ts.ctx, phoneToken)
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)

	_, err = ts.repo.FetchOne(ts.ctx, confirmedToken)
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)

	_, err = ts.repo.FetchOne(ts.ctx, otherUserToken)
	ts.Require().NoError(err, "операция другого пользователя остаётся")

	types, err = ts.repo.DeleteByUserID(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Empty(types)
}

// TestFetchByUserIDAndTypes - отдаёт действующие операции пользователя указанных типов в любом
// статусе (в т.ч. подтверждённые, ждущие применения), не отдаёт операции других типов, истёкшие
// и операции другого пользователя; у пользователя без операций - пустой срез без ошибки.
func (ts *SecureOperationPostgresTestSuite) TestFetchByUserIDAndTypes() {
	userID := uuid.New()
	types := []operationtype.Enum{operationtype.ChangePhone, operationtype.ChangeEmailConfirm, operationtype.Disable2FA}

	openedToken := ts.seedOperation(userID, operationtype.ChangePhone)
	confirmedToken := ts.seedOperation(userID, operationtype.ChangeEmailConfirm)
	expiredToken := ts.seedOperation(userID, operationtype.Disable2FA)
	ts.seedOperation(userID, operationtype.ChangePassword)
	ts.seedOperation(uuid.New(), operationtype.ChangePhone)

	// подтверждённая операция: звенья пройдены, ждёт применения
	confirmed, err := ts.repo.FetchOne(ts.ctx, confirmedToken)
	ts.Require().NoError(err)
	isConfirmed, err := confirmed.ConfirmAction(func(secureoperation.ConfirmAction) (bool, error) { return true, nil })
	ts.Require().NoError(err)
	ts.Require().True(isConfirmed)
	ts.Require().NoError(ts.repo.Replace(ts.ctx, confirmedToken, confirmed))

	// истёкшая операция
	ts.Require().NoError(ts.pgt.ConnManager().Conn(ts.ctx).Exec(
		ts.ctx,
		`UPDATE `+secureOperationsTableName+` SET expires_at = NOW() - INTERVAL '1 minute' WHERE operation_token = $1;`,
		expiredToken,
	))

	rows, err := ts.repo.FetchByUserIDAndTypes(ts.ctx, userID, types)
	ts.Require().NoError(err)

	tokens := make(map[string]secureoperation.SecureOperation, len(rows))
	for _, row := range rows {
		tokens[row.Token] = row
	}

	ts.Require().Len(tokens, 2)
	ts.Require().Contains(tokens, openedToken)
	ts.Require().Contains(tokens, confirmedToken)
	opened, confirmedRow := tokens[openedToken], tokens[confirmedToken]
	ts.True(opened.Is(operationstatus.Opened))
	ts.Equal(userID, opened.UserID)
	ts.Require().Len(opened.Actions(), 1)
	ts.True(confirmedRow.Is(operationstatus.Confirmed))

	rows, err = ts.repo.FetchByUserIDAndTypes(ts.ctx, uuid.New(), types)
	ts.Require().NoError(err)
	ts.Empty(rows)
}
