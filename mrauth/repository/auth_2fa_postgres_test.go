package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/mondegor/go-storage/mrtests/infra"
	sysmesserrors "github.com/mondegor/go-sysmess/errors"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/tests"
)

const auth2faTableName = "sample_schema.users_auth_2fa"

type Auth2faPostgresTestSuite struct {
	suite.Suite

	ctx       context.Context
	pgt       *infra.PostgresTester
	tableName string
}

func TestAuth2faPostgresTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(Auth2faPostgresTestSuite))
}

func (ts *Auth2faPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = infra.NewPostgresTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(tests.AppWorkDir() + "/mrauth/_sample/migrations")

	ts.tableName = auth2faTableName
}

func (ts *Auth2faPostgresTestSuite) TearDownSuite() {
	ts.pgt.Destroy(ts.ctx)
}

func (ts *Auth2faPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.ctx)
}

// seedUser - вставляет запись в users и возвращает её user_id.
func (ts *Auth2faPostgresTestSuite) seedUser() uuid.UUID {
	userID := uuid.New()

	sql := `
		INSERT INTO sample_schema.users
			(user_id, user_email, lang_code, user_status)
		VALUES
			($1, $2, $3, $4);`

	err := ts.pgt.ConnManager().Conn(ts.ctx).Exec(
		ts.ctx,
		sql,
		userID,
		userID.String()+"@localhost",
		"ru-RU",
		2, // ENABLED
	)
	ts.Require().NoError(err)

	return userID
}

func (ts *Auth2faPostgresTestSuite) TestRecoveryCodesRoundTrip() {
	userID := ts.seedUser()
	repo := repository.NewAuth2faPostgres(ts.pgt.ConnManager(), ts.tableName)

	err := repo.InsertOrUpdate(ts.ctx, entity.Auth2fa{
		UserID:        userID,
		Type:          auth2fatype.TOTP,
		Secret:        "SECRET",
		RecoveryCodes: []string{"hash1", "hash2", "hash3"},
	})
	ts.Require().NoError(err)

	got, err := repo.FetchOne(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Equal([]string{"hash1", "hash2", "hash3"}, got.RecoveryCodes)

	// расходование одного кода удаляет ровно один элемент
	err = repo.ConsumeRecoveryCode(ts.ctx, userID, "hash1")
	ts.Require().NoError(err)

	got, err = repo.FetchOne(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Equal([]string{"hash2", "hash3"}, got.RecoveryCodes)

	// повторное расходование того же кода (гонка) не находит запись
	err = repo.ConsumeRecoveryCode(ts.ctx, userID, "hash1")
	ts.Require().ErrorIs(err, sysmesserrors.ErrEventStorageNoRecordFound)

	got, err = repo.FetchOne(ts.ctx, userID)
	ts.Require().NoError(err)
	ts.Equal([]string{"hash2", "hash3"}, got.RecoveryCodes)
}
