package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	sysmesserrors "github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-storage/mrtests/pgtest"
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
	pgt  *pgtest.Tester
	repo *repository.SecureOperationPostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestSecureOperationPostgresTestSuite(t *testing.T) {
	suite.Run(t, new(SecureOperationPostgresTestSuite))
}

func (ts *SecureOperationPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrauth"))
	ts.repo = repository.NewSecureOperationPostgres(ts.pgt.ConnManager(), secureOperationsTableName)
}

func (ts *SecureOperationPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// Test_Insert - операция сохраняется и читается без искажений: звенья цепочки живут
// в jsonb-колонке confirm_actions, остальные поля - в своих колонках.
func (ts *SecureOperationPostgresTestSuite) Test_Insert() {
	op := ts.newOperation("token-insert", []secureoperation.ConfirmAction{
		{
			Method:      confirmmethod.Email,
			CodeLength:  6,
			MaxAttempts: 3,
			MaxResends:  5,
			Expiry:      72 * time.Hour,
			Address:     "new@example.com",
			ConfirmCode: "hash",
		},
	})

	ts.Require().NoError(ts.repo.Insert(ts.ctx, op))

	stored, err := ts.repo.FetchOne(ts.ctx, "token-insert")
	ts.Require().NoError(err)
	ts.Equal(op, stored)
}

// Test_InsertWhenChain - цепочка из нескольких звеньев сохраняется целиком и в исходном порядке,
// а признак AllowRecovery, невидимый в нулевом значении (omitempty), читается снятым там,
// где он снят, и выставленным там, где выставлен.
//
// Пара "AllowRecovery на звене RECOVERY" здесь синтетическая: ни одна фабрика такую не строит
// (аварийный код вместо аварийного кода), но она единственная, где признак можно выставить
// не нарушив инвариант - поднять его на первое звено checkInvariants не даст.
func (ts *SecureOperationPostgresTestSuite) Test_InsertWhenChain() {
	op := ts.newOperation("token-chain", []secureoperation.ConfirmAction{
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
	})

	ts.Require().NoError(ts.repo.Insert(ts.ctx, op))

	stored, err := ts.repo.FetchOne(ts.ctx, "token-chain")
	ts.Require().NoError(err)
	ts.Require().Len(stored.Actions(), 2)
	ts.Equal(confirmmethod.TOTP, stored.Actions()[0].Method)
	ts.False(stored.Actions()[0].AllowRecovery)
	ts.Equal(confirmmethod.Recovery, stored.Actions()[1].Method)
	ts.True(stored.Actions()[1].AllowRecovery)
}

// Test_Replace - открытая операция заменяется переданной целиком, включая токен: по старому
// токену операция больше не находится, по новому читается без искажений.
func (ts *SecureOperationPostgresTestSuite) Test_Replace() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/SecureOperation/Replace")

	op := ts.newReplacement("token-new")

	ts.Require().NoError(ts.repo.Replace(ts.ctx, "token-old", op))

	stored, err := ts.repo.FetchOne(ts.ctx, "token-new")
	ts.Require().NoError(err)
	ts.Equal(op, stored)

	_, err = ts.repo.FetchOne(ts.ctx, "token-old")
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)
}

// Test_ReplaceWhenConfirmed - подтверждённая операция не заменяется: возвращается
// ErrEventStorageNoRecordFound, строка остаётся прежней.
func (ts *SecureOperationPostgresTestSuite) Test_ReplaceWhenConfirmed() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/SecureOperation/Replace")

	err := ts.repo.Replace(ts.ctx, "token-confirmed", ts.newReplacement("token-new"))
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)

	stored, err := ts.repo.FetchOne(ts.ctx, "token-confirmed")
	ts.Require().NoError(err)
	ts.True(stored.Is(operationstatus.Confirmed))

	_, err = ts.repo.FetchOne(ts.ctx, "token-new")
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)
}

// Test_ReplaceWhenNotExists - замена неизвестной операции возвращает ErrEventStorageNoRecordFound.
func (ts *SecureOperationPostgresTestSuite) Test_ReplaceWhenNotExists() {
	err := ts.repo.Replace(ts.ctx, "token-unknown", ts.newReplacement("token-new"))
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)
}

// newOperation - собирает открытую операцию пользователя A со временем,
// усечённым до точности timestamptz (микросекунды).
func (ts *SecureOperationPostgresTestSuite) newOperation(token string, actions []secureoperation.ConfirmAction) secureoperation.SecureOperation {
	op, err := secureoperation.NewOperation(token, operationtype.AuthorizeUser, uuid.MustParse(fixtureUserA), actions, nil)
	ts.Require().NoError(err)

	op.ResendsAt = op.ResendsAt.Truncate(time.Microsecond)
	op.ExpiresAt = op.ExpiresAt.Truncate(time.Microsecond)

	return op
}

// newReplacement - собирает операцию, которой заменяется операция фикстуры: звено и счётчики
// отличаются от фикстуры, чтобы замена каждого поля была наблюдаема.
func (ts *SecureOperationPostgresTestSuite) newReplacement(token string) secureoperation.SecureOperation {
	op := ts.newOperation(token, []secureoperation.ConfirmAction{
		{
			Method:      confirmmethod.Password,
			MaxAttempts: 4,
			Expiry:      20 * time.Minute,
		},
	})
	op.RemainingAttempts = 1

	return op
}

// Test_DeleteByUserIDAndTypes - вытесняет все операции указанных типов пользователя, возвращает
// их типы (по одному на операцию) и не трогает ни операции других типов, ни операции тех же
// типов другого пользователя. Когда вытеснять нечего - пустой срез без ошибки.
func (ts *SecureOperationPostgresTestSuite) Test_DeleteByUserIDAndTypes() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/SecureOperation/DeleteByUserIDAndTypes")

	userID := uuid.MustParse(fixtureUserA)
	types := []operationtype.Enum{operationtype.ChangeEmail, operationtype.ChangeEmailConfirm}

	deleted, err := ts.repo.DeleteByUserIDAndTypes(ts.ctx, userID, types)
	ts.Require().NoError(err)
	ts.ElementsMatch(
		[]operationtype.Enum{operationtype.ChangeEmail, operationtype.ChangeEmail, operationtype.ChangeEmailConfirm},
		deleted,
	)

	for _, token := range []string{"token-change-email-1", "token-change-email-2", "token-change-email-confirm"} {
		_, err = ts.repo.FetchOne(ts.ctx, token)
		ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)
	}

	_, err = ts.repo.FetchOne(ts.ctx, "token-disable-2fa")
	ts.Require().NoError(err, "операция другого типа того же пользователя остаётся")

	_, err = ts.repo.FetchOne(ts.ctx, "token-other-user")
	ts.Require().NoError(err, "операция того же типа другого пользователя остаётся")

	deleted, err = ts.repo.DeleteByUserIDAndTypes(ts.ctx, userID, types)
	ts.Require().NoError(err)
	ts.Empty(deleted)
}

// Test_DeleteByUserID - удаляет все операции пользователя любого типа и в любом статусе, включая
// подтверждённые, ждущие применения (иначе такую операцию можно было бы применить после отзыва),
// и возвращает их типы (по одному на операцию); операции другого пользователя не трогает.
// У пользователя без операций - пустой срез без ошибки: отзывать нечего - штатный случай.
func (ts *SecureOperationPostgresTestSuite) Test_DeleteByUserID() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/SecureOperation/DeleteByUserID")

	userID := uuid.MustParse(fixtureUserA)

	types, err := ts.repo.DeleteByUserID(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.ElementsMatch([]operationtype.Enum{operationtype.AuthorizeUser, operationtype.ChangePhone, operationtype.ChangePhone}, types)

	_, err = ts.repo.FetchOne(ts.ctx, "token-login")
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)

	_, err = ts.repo.FetchOne(ts.ctx, "token-phone")
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)

	_, err = ts.repo.FetchOne(ts.ctx, "token-phone-confirmed")
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)

	_, err = ts.repo.FetchOne(ts.ctx, "token-other-user")
	ts.Require().NoError(err, "операция другого пользователя остаётся")

	types, err = ts.repo.DeleteByUserID(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Empty(types)
}

// Test_FetchByUserIDAndTypes - отдаёт действующие операции пользователя указанных типов в любом
// статусе (в т.ч. подтверждённые, ждущие применения), не отдаёт операции других типов, истёкшие
// и операции другого пользователя; у пользователя без операций - пустой срез без ошибки.
func (ts *SecureOperationPostgresTestSuite) Test_FetchByUserIDAndTypes() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/SecureOperation/FetchByUserIDAndTypes")

	userID := uuid.MustParse(fixtureUserA)
	types := []operationtype.Enum{operationtype.ChangePhone, operationtype.ChangeEmailConfirm, operationtype.Disable2FA}
	openedToken, confirmedToken := "token-opened", "token-confirmed"

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

	rows, err = ts.repo.FetchByUserIDAndTypes(ts.ctx, uuid.MustParse(fixtureUserC), types)
	ts.Require().NoError(err)
	ts.Empty(rows)
}

// Test_FetchOneForUpdate - внутри транзакции возвращает ту же операцию, что и FetchOne, и блокирует
// её строку до конца транзакции; неизвестный токен - ErrEventStorageNoRecordFound.
func (ts *SecureOperationPostgresTestSuite) Test_FetchOneForUpdate() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/SecureOperation/FetchOneForUpdate")

	expected, err := ts.repo.FetchOne(ts.ctx, "token-opened")
	ts.Require().NoError(err)

	err = ts.pgt.ConnManager().Do(ts.ctx, func(ctx context.Context) error {
		got, err := ts.repo.FetchOneForUpdate(ctx, "token-opened")
		ts.Require().NoError(err)
		ts.Equal(expected, got)

		// соединение вне транзакции не может захватить заблокированную строку
		var token string

		err = ts.pgt.ConnManager().Conn(ts.ctx).QueryRow(
			ts.ctx,
			`SELECT operation_token FROM `+secureOperationsTableName+` WHERE operation_token = $1 FOR UPDATE NOWAIT;`,
			"token-opened",
		).Scan(&token)

		var pgErr *pgconn.PgError

		ts.Require().ErrorAs(err, &pgErr)
		ts.Equal("55P03", pgErr.Code) // lock_not_available

		_, err = ts.repo.FetchOneForUpdate(ctx, "token-unknown")
		ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)

		return nil
	})
	ts.Require().NoError(err)
}

// Test_UpdateFailedAttempt - у открытой операции расходуется попытка подтверждения и возвращается
// остаток; у подтверждённой и неизвестной операций - ErrEventStorageNoRecordFound.
func (ts *SecureOperationPostgresTestSuite) Test_UpdateFailedAttempt() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/SecureOperation/UpdateFailedAttempt")

	attempts, err := ts.repo.UpdateFailedAttempt(ts.ctx, "token-opened")
	ts.Require().NoError(err)
	ts.Equal(int16(2), attempts)

	stored, err := ts.repo.FetchOne(ts.ctx, "token-opened")
	ts.Require().NoError(err)
	ts.Equal(int16(2), stored.RemainingAttempts)

	_, err = ts.repo.UpdateFailedAttempt(ts.ctx, "token-confirmed")
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)

	_, err = ts.repo.UpdateFailedAttempt(ts.ctx, "token-unknown")
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)
}

// Test_Delete - удаляет операцию по токену, не трогая остальные; повторное удаление (операцию
// уже потребил конкурентный запрос) возвращает ErrEventStorageNoRecordFound.
func (ts *SecureOperationPostgresTestSuite) Test_Delete() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/SecureOperation/Delete")

	ts.Require().NoError(ts.repo.Delete(ts.ctx, "token-first"))

	_, err := ts.repo.FetchOne(ts.ctx, "token-first")
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)

	_, err = ts.repo.FetchOne(ts.ctx, "token-second")
	ts.Require().NoError(err)

	err = ts.repo.Delete(ts.ctx, "token-first")
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)
}

// Test_DeleteExpired - просроченные операции удаляются пачками не более limit, начиная с самых
// старых; действующая операция остаётся.
func (ts *SecureOperationPostgresTestSuite) Test_DeleteExpired() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/SecureOperation/DeleteExpired")

	count, err := ts.repo.DeleteExpired(ts.ctx, 2)
	ts.Require().NoError(err)
	ts.Equal(2, count)
	ts.Equal([]string{"token-expired-new", "token-live"}, ts.fetchTokens())

	count, err = ts.repo.DeleteExpired(ts.ctx, 2)
	ts.Require().NoError(err)
	ts.Equal(1, count)
	ts.Equal([]string{"token-live"}, ts.fetchTokens())
}

// fetchTokens - токены всех операций таблицы в алфавитном порядке (FetchOne просроченные не отдаёт).
func (ts *SecureOperationPostgresTestSuite) fetchTokens() []string {
	rows, err := ts.pgt.ConnManager().Conn(ts.ctx).Query(
		ts.ctx,
		`SELECT operation_token FROM `+secureOperationsTableName+` ORDER BY operation_token;`,
	)
	ts.Require().NoError(err)

	defer rows.Close()

	var tokens []string

	for rows.Next() {
		var token string

		ts.Require().NoError(rows.Scan(&token))

		tokens = append(tokens, token)
	}

	ts.Require().NoError(rows.Err())

	return tokens
}

// Test_FetchOne - операция читается по токену без искажений: поля из своих колонок,
// звенья цепочки - из jsonb-колонки confirm_actions.
func (ts *SecureOperationPostgresTestSuite) Test_FetchOne() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/SecureOperation/FetchOne")

	got, err := ts.repo.FetchOne(ts.ctx, "token-opened")
	ts.Require().NoError(err)

	ts.Equal("token-opened", got.Token)
	ts.Equal(operationtype.ChangePhone, got.Type)
	ts.Equal(uuid.MustParse(fixtureUserA), got.UserID)
	ts.Equal(int16(2), got.RemainingAttempts)
	ts.Equal(int16(4), got.RemainingResends)
	ts.Equal(time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC), got.ResendsAt)
	ts.Equal(time.Date(2099, 1, 2, 0, 0, 0, 0, time.UTC), got.ExpiresAt)
	ts.True(got.Is(operationstatus.Opened))
	ts.Equal(
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
		got.Actions(),
	)
}

// Test_FetchOneWhenAnonymous - у операции без владельца (user_id = NULL) UserID читается нулевым.
func (ts *SecureOperationPostgresTestSuite) Test_FetchOneWhenAnonymous() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/SecureOperation/FetchOne")

	got, err := ts.repo.FetchOne(ts.ctx, "token-anonymous")
	ts.Require().NoError(err)
	ts.Equal(operationtype.CreateUser, got.Type)
	ts.Equal(uuid.Nil, got.UserID)
}

// Test_FetchOneWhenExpired - истёкшая операция не выдаётся: возвращается mrauth.ErrOperationAlreadyExpired.
func (ts *SecureOperationPostgresTestSuite) Test_FetchOneWhenExpired() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/SecureOperation/FetchOne")

	_, err := ts.repo.FetchOne(ts.ctx, "token-expired")
	ts.Require().ErrorIs(err, mrauth.ErrOperationAlreadyExpired)
}

// Test_FetchOneWhenNotExists - неизвестный токен возвращает ErrEventStorageNoRecordFound.
func (ts *SecureOperationPostgresTestSuite) Test_FetchOneWhenNotExists() {
	_, err := ts.repo.FetchOne(ts.ctx, "token-unknown")
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)
}
