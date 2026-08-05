package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-storage/mrtests/infra"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/authtokentype"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/tests"
)

// authTokensTableName объявлена в session_cleanup_postgres_test.go.

type AuthTokenPostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *infra.PostgresTester
	repo *repository.AuthTokenPostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestAuthTokenPostgresTestSuite(t *testing.T) {
	suite.Run(t, new(AuthTokenPostgresTestSuite))
}

func (ts *AuthTokenPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = infra.NewPostgresTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(tests.AppWorkDir() + "/mrauth/_sample/migrations")
	ts.repo = repository.NewAuthTokenPostgres(ts.pgt.ConnManager(), authTokensTableName)
}

func (ts *AuthTokenPostgresTestSuite) TearDownSuite() {
	ts.pgt.Destroy(ts.ctx)
}

func (ts *AuthTokenPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.ctx)
}

// seedSession - сохраняет пару токенов одной сессии и возвращает их значения.
func (ts *AuthTokenPostgresTestSuite) seedSession(userID uuid.UUID, sessionID uint32) (accessToken, refreshToken string) {
	return ts.seedSessionWithScopes(userID, sessionID, entity.AuthTokenScopes{})
}

// seedSessionWithScopes - сохраняет пару токенов одной сессии с указанной областью действия.
func (ts *AuthTokenPostgresTestSuite) seedSessionWithScopes(
	userID uuid.UUID,
	sessionID uint32,
	scopes entity.AuthTokenScopes,
) (accessToken, refreshToken string) {
	accessToken = "access-" + uuid.NewString()
	refreshToken = "refresh-" + uuid.NewString()
	expiresAt := time.Now().UTC().Add(time.Hour)

	err := ts.repo.Insert(ts.ctx, []entity.AuthToken{
		{
			Token:     accessToken,
			Type:      authtokentype.Access,
			UserID:    userID,
			RealmID:   1,
			SessionID: sessionID,
			Scopes:    scopes,
			ExpiresAt: expiresAt,
		},
		{
			Token:     refreshToken,
			Type:      authtokentype.Refresh,
			UserID:    userID,
			RealmID:   1,
			SessionID: sessionID,
			Scopes:    scopes,
			ExpiresAt: expiresAt,
		},
	})
	ts.Require().NoError(err)

	return accessToken, refreshToken
}

// TestUpdateScopesSettings - новые язык и пояс попадают во все действующие refresh токены
// пользователя (во всех его сессиях), оставляя нетронутыми realm и вид пользователя,
// а область действия выданных access токенов сохраняется прежней: до их истечения
// ответы формируются по зафиксированным в них значениям.
func (ts *AuthTokenPostgresTestSuite) TestUpdateScopesSettings() {
	userID := uuid.New()
	scopes := entity.AuthTokenScopes{Realm: "app/users", UserKind: "regular", LangCode: "ru-RU", TimeZone: "Europe/Moscow"}

	ts.seedSessionWithScopes(userID, 1, scopes)
	ts.seedSessionWithScopes(userID, 2, scopes)

	// чужие токены обновляться не должны
	otherUserID := uuid.New()
	ts.seedSessionWithScopes(otherUserID, 1, scopes)

	ts.Require().NoError(ts.repo.UpdateScopesSettings(ts.ctx, userID, "en-US", "Asia/Tokyo"))

	for _, sessionID := range []uint32{1, 2} {
		access, refresh, err := ts.repo.FetchLastEnabledPairBySessionID(ts.ctx, userID, sessionID)
		ts.Require().NoError(err)

		ts.Equal(
			entity.AuthTokenScopes{Realm: "app/users", UserKind: "regular", LangCode: "en-US", TimeZone: "Asia/Tokyo"},
			refresh.Scopes,
		)
		ts.Equal(scopes, access.Scopes)
	}

	_, refresh, err := ts.repo.FetchLastEnabledPairBySessionID(ts.ctx, otherUserID, 1)
	ts.Require().NoError(err)
	ts.Equal(scopes, refresh.Scopes)
}

// TestUpdateScopesSettingsWithoutSessions - пользователь без открытых сессий не считается
// ошибкой: настройки сохраняются, применять их просто некуда.
func (ts *AuthTokenPostgresTestSuite) TestUpdateScopesSettingsWithoutSessions() {
	ts.Require().NoError(ts.repo.UpdateScopesSettings(ts.ctx, uuid.New(), "en-US", "Asia/Tokyo"))
}

// TestUpdateScopesSettingsSkipsRevoked - отозванные токены не обновляются:
// закрытая сессия не должна ожить с новыми настройками.
func (ts *AuthTokenPostgresTestSuite) TestUpdateScopesSettingsSkipsRevoked() {
	userID := uuid.New()
	scopes := entity.AuthTokenScopes{Realm: "app/users", UserKind: "regular", LangCode: "ru-RU", TimeZone: "Europe/Moscow"}

	_, refreshToken := ts.seedSessionWithScopes(userID, 1, scopes)
	ts.Require().NoError(ts.repo.RevokeSessionByRefreshToken(ts.ctx, refreshToken))

	ts.Require().NoError(ts.repo.UpdateScopesSettings(ts.ctx, userID, "en-US", "Asia/Tokyo"))

	_, _, err := ts.repo.FetchLastEnabledPairBySessionID(ts.ctx, userID, 1)
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

func (ts *AuthTokenPostgresTestSuite) TestRevokeSessionByRefreshToken() {
	userID := uuid.New()
	accessToken, refreshToken := ts.seedSession(userID, 1)

	// до logout access токен сессии действует
	_, err := ts.repo.FetchOneByAccessToken(ts.ctx, accessToken)
	ts.Require().NoError(err)

	ts.Require().NoError(ts.repo.RevokeSessionByRefreshToken(ts.ctx, refreshToken))

	// logout отзывает все токены сессии, а не только refresh
	_, err = ts.repo.FetchOneByAccessToken(ts.ctx, accessToken)
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)

	// повторному logout отзывать уже нечего: на этом построена идемпотентность AuthToken.Close
	err = ts.repo.RevokeSessionByRefreshToken(ts.ctx, refreshToken)
	ts.Require().ErrorIs(err, errors.ErrEventStorageRecordsNotAffected)

	// неизвестный refresh токен ведёт себя так же
	err = ts.repo.RevokeSessionByRefreshToken(ts.ctx, "refresh-unknown")
	ts.Require().ErrorIs(err, errors.ErrEventStorageRecordsNotAffected)
}

func (ts *AuthTokenPostgresTestSuite) TestRevokeSessionByRefreshTokenKeepsOtherSessions() {
	userID := uuid.New()
	_, refreshFirst := ts.seedSession(userID, 1)
	_, refreshSecond := ts.seedSession(userID, 2)

	ts.Require().NoError(ts.repo.RevokeSessionByRefreshToken(ts.ctx, refreshFirst))

	// logout закрывает только свою сессию, вторая остаётся действующей
	count, err := ts.repo.FetchOpenSessionCount(ts.ctx, userID, 1)
	ts.Require().NoError(err)
	ts.Equal(1, count)

	ts.Require().NoError(ts.repo.RevokeSessionByRefreshToken(ts.ctx, refreshSecond))

	count, err = ts.repo.FetchOpenSessionCount(ts.ctx, userID, 1)
	ts.Require().NoError(err)
	ts.Equal(0, count)
}
