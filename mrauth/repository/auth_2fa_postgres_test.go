package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	sysmesserrors "github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-storage/mrtests/infra"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/tests"
)

const auth2faTableName = "sample_schema.users_auth_2fa"

type Auth2FAPostgresTestSuite struct {
	suite.Suite

	ctx       context.Context
	pgt       *infra.PostgresTester
	tableName string
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestAuth2FAPostgresTestSuite(t *testing.T) {
	suite.Run(t, new(Auth2FAPostgresTestSuite))
}

func (ts *Auth2FAPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = infra.NewPostgresTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(tests.AppWorkDir() + "/mrauth/_sample/migrations")

	ts.tableName = auth2faTableName
}

func (ts *Auth2FAPostgresTestSuite) TearDownSuite() {
	ts.pgt.Destroy(ts.ctx)
}

func (ts *Auth2FAPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.ctx)
}

// seedUser - вставляет запись в users и возвращает её user_id.
func (ts *Auth2FAPostgresTestSuite) seedUser() uuid.UUID {
	userID := uuid.New()

	sql := `
		INSERT INTO sample_schema.users
			(user_id, user_email, lang_code, user_timezone, registered_ip, user_status)
		VALUES
			($1, $2, $3, $4, $5, $6);`

	err := ts.pgt.ConnManager().Conn(ts.ctx).Exec(
		ts.ctx,
		sql,
		userID,
		userID.String()+"@localhost",
		"ru-RU",
		"Europe/Moscow",
		"203.0.113.7",
		2, // ENABLED
	)
	ts.Require().NoError(err)

	return userID
}

func (ts *Auth2FAPostgresTestSuite) TestRecoveryCodesRoundTrip() {
	userID := ts.seedUser()
	repo := repository.NewAuth2FAPostgres(ts.pgt.ConnManager(), ts.tableName)

	err := repo.Insert(ts.ctx, entity.Auth2FA{
		UserID:        userID,
		Type:          auth2fatype.TOTP,
		Secret:        "SECRET",
		RecoveryCodes: []string{"hash1", "hash2", "hash3"},
	})
	ts.Require().NoError(err)

	got, err := repo.FetchOne(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Equal([]string{"hash1", "hash2", "hash3"}, got.RecoveryCodes)

	// расходование одного кода удаляет ровно один элемент и возвращает остаток
	remaining, err := repo.UpdateRecoveryCode(ts.ctx, userID, "hash1")
	ts.Require().NoError(err)
	ts.Equal(2, remaining)

	got, err = repo.FetchOne(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Equal([]string{"hash2", "hash3"}, got.RecoveryCodes)

	// повторное расходование того же кода (гонка) не находит запись
	_, err = repo.UpdateRecoveryCode(ts.ctx, userID, "hash1")
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)

	got, err = repo.FetchOne(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Equal([]string{"hash2", "hash3"}, got.RecoveryCodes)
}

// TestInsertOnActive2FAConflicts - повторная привязка при уже активном 2FA отклоняется
// нарушением уникальности, а не перезаписывает текущий второй фактор. На этом построена
// защита apply-password/apply-totp от гонки «2FA включили другим способом между созданием
// операции и её применением» (mrauth.ErrAuth2FAMustBeDisabledFirst -> 409).
func (ts *Auth2FAPostgresTestSuite) TestInsertOnActive2FAConflicts() {
	userID := ts.seedUser()
	repo := repository.NewAuth2FAPostgres(ts.pgt.ConnManager(), ts.tableName)

	err := repo.Insert(ts.ctx, entity.Auth2FA{
		UserID:        userID,
		Type:          auth2fatype.TOTP,
		Secret:        "TOTP-SECRET",
		RecoveryCodes: []string{"hash1"},
	})
	ts.Require().NoError(err)

	err = repo.Insert(ts.ctx, entity.Auth2FA{
		UserID:        userID,
		Type:          auth2fatype.Password,
		Secret:        "PASSWORD-HASH",
		RecoveryCodes: []string{"other-hash"},
	})
	ts.Require().ErrorIs(err, sysmesserrors.ErrInternalStorageDuplicateKeyViolation)

	// активный второй фактор остался нетронутым
	got, err := repo.FetchOne(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Equal(auth2fatype.TOTP, got.Type)
	ts.Equal("TOTP-SECRET", got.Secret)
	ts.Equal([]string{"hash1"}, got.RecoveryCodes)
}

func (ts *Auth2FAPostgresTestSuite) TestDelete() {
	userID := ts.seedUser()
	repo := repository.NewAuth2FAPostgres(ts.pgt.ConnManager(), ts.tableName)

	err := repo.Insert(ts.ctx, entity.Auth2FA{
		UserID:        userID,
		Type:          auth2fatype.TOTP,
		Secret:        "SECRET",
		RecoveryCodes: []string{"hash1"},
	})
	ts.Require().NoError(err)

	ts.Require().NoError(repo.Delete(ts.ctx, userID))

	_, err = repo.FetchOne(ts.ctx, userID)
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)

	// повторное удаление сообщает об отсутствии записи: на этом построена
	// идемпотентность обработчика отключения 2FA
	err = repo.Delete(ts.ctx, userID)
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)

	// удаление по неизвестному пользователю ведёт себя так же
	err = repo.Delete(ts.ctx, uuid.New())
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)
}

func (ts *Auth2FAPostgresTestSuite) TestUpdateTOTPStepMonotonic() {
	userID := ts.seedUser()
	repo := repository.NewAuth2FAPostgres(ts.pgt.ConnManager(), ts.tableName)

	err := repo.Insert(ts.ctx, entity.Auth2FA{
		UserID:        userID,
		Type:          auth2fatype.TOTP,
		Secret:        "SECRET",
		RecoveryCodes: []string{},
		LastTOTPStep:  100,
	})
	ts.Require().NoError(err)

	got, err := repo.FetchOne(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Equal(int64(100), got.LastTOTPStep)

	// шаг сдвигается вперёд только при строго большем значении
	ts.Require().NoError(repo.UpdateTOTPStep(ts.ctx, userID, 101))

	got, err = repo.FetchOne(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Equal(int64(101), got.LastTOTPStep)

	// повтор того же шага (replay) отклоняется и не меняет значение
	err = repo.UpdateTOTPStep(ts.ctx, userID, 101)
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)

	// более старый шаг также отклоняется
	err = repo.UpdateTOTPStep(ts.ctx, userID, 50)
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)

	got, err = repo.FetchOne(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Equal(int64(101), got.LastTOTPStep)
}
