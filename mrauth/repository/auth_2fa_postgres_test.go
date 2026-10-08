package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	sysmesserrors "github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-storage/mrtests/pgtest"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/tests"
)

const auth2faTableName = "sample_schema.users_auth_2fa"

type Auth2FAPostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *pgtest.Tester
	repo *repository.Auth2FAPostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestAuth2FAPostgresTestSuite(t *testing.T) {
	suite.Run(t, new(Auth2FAPostgresTestSuite))
}

func (ts *Auth2FAPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrauth"))

	ts.repo = repository.NewAuth2FAPostgres(ts.pgt.ConnManager(), auth2faTableName)
}

func (ts *Auth2FAPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// Test_UpdateRecoveryCode - аварийные коды сохраняются и читаются массивом,
// расходование кода удаляет ровно один элемент.
func (ts *Auth2FAPostgresTestSuite) Test_UpdateRecoveryCode() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Auth2FA/UpdateRecoveryCode")

	userID := uuid.MustParse(fixtureUserA)

	err := ts.repo.Insert(ts.ctx, entity.Auth2FA{
		UserID:        userID,
		Type:          auth2fatype.TOTP,
		Secret:        "SECRET",
		RecoveryCodes: []string{"hash1", "hash2", "hash3"},
	})
	ts.Require().NoError(err)

	got, err := ts.repo.FetchOne(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Equal([]string{"hash1", "hash2", "hash3"}, got.RecoveryCodes)

	// расходование одного кода удаляет ровно один элемент и возвращает остаток
	remaining, err := ts.repo.UpdateRecoveryCode(ts.ctx, userID, "hash1")
	ts.Require().NoError(err)
	ts.Equal(2, remaining)

	got, err = ts.repo.FetchOne(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Equal([]string{"hash2", "hash3"}, got.RecoveryCodes)

	// повторное расходование того же кода (гонка) не находит запись
	_, err = ts.repo.UpdateRecoveryCode(ts.ctx, userID, "hash1")
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)

	got, err = ts.repo.FetchOne(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Equal([]string{"hash2", "hash3"}, got.RecoveryCodes)
}

// Test_InsertWhen2FAIsActive - повторная привязка при уже активном 2FA отклоняется
// нарушением уникальности, а не перезаписывает текущий второй фактор. На этом построена
// защита apply-password/apply-totp от гонки «2FA включили другим способом между созданием
// операции и её применением» (mrauth.ErrAuth2FAMustBeDisabledFirst -> 409).
func (ts *Auth2FAPostgresTestSuite) Test_InsertWhen2FAIsActive() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Auth2FA/InsertWhen2FAIsActive")

	userID := uuid.MustParse(fixtureUserA)

	err := ts.repo.Insert(ts.ctx, entity.Auth2FA{
		UserID:        userID,
		Type:          auth2fatype.Password,
		Secret:        "PASSWORD-HASH",
		RecoveryCodes: []string{"other-hash"},
	})
	ts.Require().ErrorIs(err, sysmesserrors.ErrInternalStorageDuplicateKeyViolation)

	// активный второй фактор остался нетронутым
	got, err := ts.repo.FetchOne(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Equal(auth2fatype.TOTP, got.Type)
	ts.Equal("TOTP-SECRET", got.Secret)
	ts.Equal([]string{"hash1"}, got.RecoveryCodes)
}

// Test_Delete - удаление привязки 2FA; повторное удаление сообщает об отсутствии записи.
func (ts *Auth2FAPostgresTestSuite) Test_Delete() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Auth2FA/Delete")

	userID := uuid.MustParse(fixtureUserA)

	ts.Require().NoError(ts.repo.Delete(ts.ctx, userID))

	_, err := ts.repo.FetchOne(ts.ctx, userID)
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)

	// повторное удаление сообщает об отсутствии записи: на этом построена
	// идемпотентность обработчика отключения 2FA
	err = ts.repo.Delete(ts.ctx, userID)
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)

	// удаление по неизвестному пользователю ведёт себя так же
	err = ts.repo.Delete(ts.ctx, uuid.MustParse(fixtureUserC))
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)
}

// Test_UpdateTOTPStep - TOTP-шаг сдвигается только вперёд (защита от replay).
func (ts *Auth2FAPostgresTestSuite) Test_UpdateTOTPStep() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Auth2FA/UpdateTOTPStep")

	userID := uuid.MustParse(fixtureUserA)

	got, err := ts.repo.FetchOne(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Equal(int64(100), got.LastTOTPStep)

	// шаг сдвигается вперёд только при строго большем значении
	ts.Require().NoError(ts.repo.UpdateTOTPStep(ts.ctx, userID, 101))

	got, err = ts.repo.FetchOne(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Equal(int64(101), got.LastTOTPStep)

	// повтор того же шага (replay) отклоняется и не меняет значение
	err = ts.repo.UpdateTOTPStep(ts.ctx, userID, 101)
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)

	// более старый шаг также отклоняется
	err = ts.repo.UpdateTOTPStep(ts.ctx, userID, 50)
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)

	got, err = ts.repo.FetchOne(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Equal(int64(101), got.LastTOTPStep)
}

// Test_UpdateRecoveryCodes - перевыпуск заменяет набор аварийных кодов целиком и сбрасывает
// время последнего расхода кода.
func (ts *Auth2FAPostgresTestSuite) Test_UpdateRecoveryCodes() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Auth2FA/UpdateRecoveryCodes")

	userID := uuid.MustParse(fixtureUserA)

	ts.Require().NoError(ts.repo.UpdateRecoveryCodes(ts.ctx, userID, []string{"new1", "new2", "new3"}))

	got, err := ts.repo.FetchOne(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Equal([]string{"new1", "new2", "new3"}, got.RecoveryCodes)

	var isRecoveryReset bool

	err = ts.pgt.ConnManager().Conn(ts.ctx).QueryRow(
		ts.ctx,
		`SELECT last_recovery_at IS NULL FROM `+auth2faTableName+` WHERE user_id = $1;`,
		userID,
	).Scan(&isRecoveryReset)
	ts.Require().NoError(err)
	ts.True(isRecoveryReset)
}

// Test_UpdateRecoveryCodesWhen2FANotExists - у пользователя без 2FA перевыпускать нечего:
// возвращается ErrEventStorageNoRecordFound.
func (ts *Auth2FAPostgresTestSuite) Test_UpdateRecoveryCodesWhen2FANotExists() {
	err := ts.repo.UpdateRecoveryCodes(ts.ctx, uuid.MustParse(fixtureUserA), []string{"new1"})
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)
}

// Test_FetchOne - данные 2FA пользователя читаются без искажений.
func (ts *Auth2FAPostgresTestSuite) Test_FetchOne() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Auth2FA/FetchOne")

	got, err := ts.repo.FetchOne(ts.ctx, uuid.MustParse(fixtureUserA))
	ts.Require().NoError(err)
	ts.Equal(
		entity.Auth2FA{
			Type:          auth2fatype.Password,
			Secret:        "PASSWORD-HASH",
			LastTOTPStep:  7,
			RecoveryCodes: []string{"hash1", "hash2"},
		},
		got,
	)
}

// Test_FetchOneWhenNotExists - у пользователя без 2FA возвращается ErrEventStorageNoRecordFound.
func (ts *Auth2FAPostgresTestSuite) Test_FetchOneWhenNotExists() {
	_, err := ts.repo.FetchOne(ts.ctx, uuid.MustParse(fixtureUserA))
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)
}
