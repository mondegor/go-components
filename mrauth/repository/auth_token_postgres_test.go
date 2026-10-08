package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-storage/mrtests/pgtest"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/authtokentype"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/tests"
)

// authTokensTableName объявлена в session_cleanup_postgres_test.go.

type AuthTokenPostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *pgtest.Tester
	repo *repository.AuthTokenPostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestAuthTokenPostgresTestSuite(t *testing.T) {
	suite.Run(t, new(AuthTokenPostgresTestSuite))
}

func (ts *AuthTokenPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrauth"))
	ts.repo = repository.NewAuthTokenPostgres(ts.pgt.ConnManager(), authTokensTableName)
}

func (ts *AuthTokenPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// Test_Insert - пара токенов сессии сохраняется одной вставкой и читается без искажений,
// включая область действия, хранимую в jsonb-колонке token_scopes.
func (ts *AuthTokenPostgresTestSuite) Test_Insert() {
	userID := uuid.MustParse(fixtureUserA)
	scopes := entity.AuthTokenScopes{Realm: "app/users", UserKind: "regular", LangCode: "ru-RU", TimeZone: "Europe/Moscow"}
	expiresAt := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)

	err := ts.repo.Insert(ts.ctx, []entity.AuthToken{
		{
			Token:     "access-a-1",
			Type:      authtokentype.Access,
			UserID:    userID,
			RealmID:   realmA,
			SessionID: 1,
			Scopes:    scopes,
			ExpiresAt: expiresAt,
		},
		{
			Token:     "refresh-a-1",
			Type:      authtokentype.Refresh,
			UserID:    userID,
			RealmID:   realmA,
			SessionID: 1,
			Scopes:    scopes,
			ExpiresAt: expiresAt,
		},
	})
	ts.Require().NoError(err)

	access, refresh, err := ts.repo.FetchLastEnabledPairBySessionID(ts.ctx, userID, 1)
	ts.Require().NoError(err)

	ts.Equal("access-a-1", access.Token)
	ts.Equal(scopes, access.Scopes)
	ts.Equal(expiresAt, access.ExpiresAt)

	ts.Equal("refresh-a-1", refresh.Token)
	ts.Equal(scopes, refresh.Scopes)
	ts.Equal(expiresAt, refresh.ExpiresAt)
}

// Test_UpdateScopesSettings - новые язык и пояс попадают во все действующие refresh токены
// пользователя (во всех его сессиях), оставляя нетронутыми realm и вид пользователя,
// а область действия выданных access токенов сохраняется прежней: до их истечения
// ответы формируются по зафиксированным в них значениям.
func (ts *AuthTokenPostgresTestSuite) Test_UpdateScopesSettings() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/UpdateScopesSettings")

	userID := uuid.MustParse(fixtureUserA)
	otherUserID := uuid.MustParse(fixtureUserB) // чужие токены обновляться не должны
	scopes := entity.AuthTokenScopes{Realm: "app/users", UserKind: "regular", LangCode: "ru-RU", TimeZone: "Europe/Moscow"}

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

// Test_UpdateScopesSettingsWhenNoSessions - пользователь без открытых сессий не считается
// ошибкой: настройки сохраняются, применять их просто некуда.
func (ts *AuthTokenPostgresTestSuite) Test_UpdateScopesSettingsWhenNoSessions() {
	ts.Require().NoError(ts.repo.UpdateScopesSettings(ts.ctx, uuid.MustParse(fixtureUserC), "en-US", "Asia/Tokyo"))
}

// Test_UpdateScopesSettingsWhenTokensRevoked - отозванные токены не обновляются:
// закрытая сессия не должна ожить с новыми настройками.
func (ts *AuthTokenPostgresTestSuite) Test_UpdateScopesSettingsWhenTokensRevoked() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/UpdateScopesSettingsWhenTokensRevoked")

	userID := uuid.MustParse(fixtureUserA)

	ts.Require().NoError(ts.repo.UpdateScopesSettings(ts.ctx, userID, "en-US", "Asia/Tokyo"))

	_, _, err := ts.repo.FetchLastEnabledPairBySessionID(ts.ctx, userID, 1)
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_RevokeSessionByRefreshToken - logout отзывает все токены сессии; повторный logout
// и неизвестный токен сообщают, что отзывать нечего.
func (ts *AuthTokenPostgresTestSuite) Test_RevokeSessionByRefreshToken() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/RevokeSessionByRefreshToken")

	userID := uuid.MustParse(fixtureUserA)

	// до logout access токен сессии действует
	_, err := ts.repo.FetchOneByAccessToken(ts.ctx, "access-a-1")
	ts.Require().NoError(err)

	ts.Require().NoError(ts.repo.RevokeSessionByRefreshToken(ts.ctx, userID, "refresh-a-1"))

	// logout отзывает все токены сессии, а не только refresh
	_, err = ts.repo.FetchOneByAccessToken(ts.ctx, "access-a-1")
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)

	// повторному logout отзывать уже нечего: на этом построена идемпотентность AuthToken.Close
	err = ts.repo.RevokeSessionByRefreshToken(ts.ctx, userID, "refresh-a-1")
	ts.Require().ErrorIs(err, errors.ErrEventStorageRecordsNotAffected)

	// неизвестный refresh токен ведёт себя так же
	err = ts.repo.RevokeSessionByRefreshToken(ts.ctx, userID, "refresh-unknown")
	ts.Require().ErrorIs(err, errors.ErrEventStorageRecordsNotAffected)
}

// Test_RevokeSessionByRefreshTokenWhenOtherUser - refresh токен чужой сессии ведёт себя как
// неизвестный: сессия владельца не закрывается, а вызывающий получает тот же sentinel,
// что и на неизвестный токен (на нём AuthToken.Close строит молчаливый 204).
func (ts *AuthTokenPostgresTestSuite) Test_RevokeSessionByRefreshTokenWhenOtherUser() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/RevokeSessionByRefreshToken")

	err := ts.repo.RevokeSessionByRefreshToken(ts.ctx, uuid.MustParse(fixtureUserB), "refresh-a-1")
	ts.Require().ErrorIs(err, errors.ErrEventStorageRecordsNotAffected)

	// сессия владельца по-прежнему действует
	_, err = ts.repo.FetchOneByAccessToken(ts.ctx, "access-a-1")
	ts.Require().NoError(err)
}

// Test_RevokeSessionByRefreshTokenWhenOtherSessionsExist - logout закрывает только свою сессию.
func (ts *AuthTokenPostgresTestSuite) Test_RevokeSessionByRefreshTokenWhenOtherSessionsExist() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/RevokeSessionByRefreshTokenWhenOtherSessionsExist")

	userID := uuid.MustParse(fixtureUserA)

	ts.Require().NoError(ts.repo.RevokeSessionByRefreshToken(ts.ctx, userID, "refresh-a-1"))

	// logout закрывает только свою сессию, вторая остаётся действующей
	count, err := ts.repo.FetchOpenSessionCount(ts.ctx, userID, 1)
	ts.Require().NoError(err)
	ts.Equal(1, count)

	ts.Require().NoError(ts.repo.RevokeSessionByRefreshToken(ts.ctx, userID, "refresh-a-2"))

	count, err = ts.repo.FetchOpenSessionCount(ts.ctx, userID, 1)
	ts.Require().NoError(err)
	ts.Equal(0, count)
}

// Test_RevokeTokensBySessionIDs - отзываются действующие токены своих сессий из списка, чужие
// не затрагиваются; если закрывать нечего (сессии чужие, неизвестные или уже закрыты),
// возвращается ErrEventStorageRecordsNotAffected.
func (ts *AuthTokenPostgresTestSuite) Test_RevokeTokensBySessionIDs() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/RevokeTokensBySessionIDs")

	userID := uuid.MustParse(fixtureUserA)
	otherUserID := uuid.MustParse(fixtureUserB)

	ts.Require().NoError(ts.repo.RevokeTokensBySessionIDs(ts.ctx, userID, []uint32{1, 2, 3, 4, 99}))

	count, err := ts.repo.FetchOpenSessionCount(ts.ctx, userID, 1)
	ts.Require().NoError(err)
	ts.Equal(0, count)

	// чужая сессия не тронута
	count, err = ts.repo.FetchOpenSessionCount(ts.ctx, otherUserID, 1)
	ts.Require().NoError(err)
	ts.Equal(1, count)

	// повторный отзыв, чужая и неизвестная сессии: закрывать нечего
	err = ts.repo.RevokeTokensBySessionIDs(ts.ctx, userID, []uint32{1, 2, 3, 4, 99})
	ts.Require().ErrorIs(err, errors.ErrEventStorageRecordsNotAffected)

	err = ts.repo.RevokeTokensBySessionIDs(ts.ctx, userID, nil)
	ts.Require().ErrorIs(err, errors.ErrEventStorageRecordsNotAffected)
}

// Test_RevokeTokensBySessionIDsWhenSessionExpired - истёкшая, но ещё не вычищенная сессия открытой
// не считается: закрывать нечего, возвращается ErrEventStorageRecordsNotAffected.
func (ts *AuthTokenPostgresTestSuite) Test_RevokeTokensBySessionIDsWhenSessionExpired() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/RevokeTokensBySessionIDsWhenSessionExpired")

	err := ts.repo.RevokeTokensBySessionIDs(ts.ctx, uuid.MustParse(fixtureUserA), []uint32{1})
	ts.Require().ErrorIs(err, errors.ErrEventStorageRecordsNotAffected)
}

// Test_FetchOpenSessions - отдаются только открытые сессии пользователя в указанном realm (есть
// действующий не истёкший refresh токен), наименее активные первыми (по времени выпуска refresh токена).
func (ts *AuthTokenPostgresTestSuite) Test_FetchOpenSessions() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/FetchOpenSessions")

	rows, err := ts.repo.FetchOpenSessions(ts.ctx, uuid.MustParse(fixtureUserA), realmA)
	ts.Require().NoError(err)
	ts.Equal(
		entity.OpenSessions{
			{SessionID: 2, ExpiresAt: time.Date(2099, 1, 2, 0, 0, 0, 0, time.UTC)},
			{SessionID: 3, ExpiresAt: time.Date(2099, 1, 3, 0, 0, 0, 0, time.UTC)},
			{SessionID: 1, ExpiresAt: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)},
		},
		rows,
	)
}

// Test_FetchOpenSessionsWhenNoSessions - у пользователя без открытых сессий - пустой срез, а не ошибка.
func (ts *AuthTokenPostgresTestSuite) Test_FetchOpenSessionsWhenNoSessions() {
	rows, err := ts.repo.FetchOpenSessions(ts.ctx, uuid.MustParse(fixtureUserA), realmA)
	ts.Require().NoError(err)
	ts.Empty(rows)
}

// Test_RevokeRefresh - действующий refresh токен отзывается с окном повторного запроса длиной grace
// и возвращает область действия сессии; повтор в пределах окна распознаётся как повторный запрос.
func (ts *AuthTokenPostgresTestSuite) Test_RevokeRefresh() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/RevokeRefresh")

	const grace = 30 * time.Second

	expected := dto.UserScopes{
		UserID:    uuid.MustParse(fixtureUserA),
		SessionID: 1,
		Realm:     "app/users",
		Kind:      "regular",
		LangCode:  "ru-RU",
		TimeZone:  "Europe/Moscow",
	}

	row, isRetried, err := ts.repo.RevokeRefresh(ts.ctx, "refresh-active", grace)
	ts.Require().NoError(err)
	ts.False(isRetried)
	ts.Equal(expected, row)

	var (
		status    int16
		expiresAt time.Time
	)

	err = ts.pgt.ConnManager().Conn(ts.ctx).QueryRow(
		ts.ctx,
		`SELECT token_status, expires_at FROM `+authTokensTableName+` WHERE auth_token = $1;`,
		"refresh-active",
	).Scan(&status, &expiresAt)
	ts.Require().NoError(err)
	ts.Equal(int16(2), status) // REVOKED
	ts.WithinDuration(time.Now().Add(grace), expiresAt, 5*time.Second)

	// повторный запрос с тем же токеном в пределах окна
	row, isRetried, err = ts.repo.RevokeRefresh(ts.ctx, "refresh-active", grace)
	ts.Require().NoError(err)
	ts.True(isRetried)
	ts.Equal(expected, row)
}

// Test_RevokeRefreshWhenWithinGrace - отозванный токен с открытым окном повторного запроса
// возвращает область действия своей сессии с признаком повторного запроса.
func (ts *AuthTokenPostgresTestSuite) Test_RevokeRefreshWhenWithinGrace() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/RevokeRefresh")

	row, isRetried, err := ts.repo.RevokeRefresh(ts.ctx, "refresh-grace", time.Minute)
	ts.Require().NoError(err)
	ts.True(isRetried)
	ts.Equal(uuid.MustParse(fixtureUserA), row.UserID)
	ts.Equal(uint32(2), row.SessionID)
}

// Test_RevokeRefreshWhenExpired - истёкший, но не отозванный токен возвращает mrauth.ErrEventTokenExpired.
func (ts *AuthTokenPostgresTestSuite) Test_RevokeRefreshWhenExpired() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/RevokeRefresh")

	_, _, err := ts.repo.RevokeRefresh(ts.ctx, "refresh-expired", time.Minute)
	ts.Require().ErrorIs(err, mrauth.ErrEventTokenExpired)
}

// Test_RevokeRefreshWhenAlreadyRevoked - отозванный токен с закрытым окном повторного запроса
// (возможная компрометация) возвращает mrauth.TokenAlreadyRevokedError с его сессией.
func (ts *AuthTokenPostgresTestSuite) Test_RevokeRefreshWhenAlreadyRevoked() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/RevokeRefresh")

	_, _, err := ts.repo.RevokeRefresh(ts.ctx, "refresh-revoked-expired", time.Minute)

	var revokedErr *mrauth.TokenAlreadyRevokedError

	ts.Require().ErrorAs(err, &revokedErr)
	ts.Equal(uuid.MustParse(fixtureUserA), revokedErr.UserID)
	ts.Equal(uint32(4), revokedErr.SessionID)
}

// Test_RevokeRefreshWhenNotFound - неизвестный токен, как и токен другого типа,
// возвращает ErrEventStorageNoRecordFound.
func (ts *AuthTokenPostgresTestSuite) Test_RevokeRefreshWhenNotFound() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/RevokeRefresh")

	_, _, err := ts.repo.RevokeRefresh(ts.ctx, "refresh-unknown", time.Minute)
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)

	_, _, err = ts.repo.RevokeRefresh(ts.ctx, "access-active", time.Minute)
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_RevokeTokensBySessionID - отзываются действующие токены только указанной сессии;
// если закрывать нечего, возвращается ErrEventStorageRecordsNotAffected.
func (ts *AuthTokenPostgresTestSuite) Test_RevokeTokensBySessionID() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/RevokeTokensBySessionID")

	userID := uuid.MustParse(fixtureUserA)

	ts.Require().NoError(ts.repo.RevokeTokensBySessionID(ts.ctx, userID, 1))

	_, err := ts.repo.FetchOneByAccessToken(ts.ctx, "access-a-1")
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)

	// вторая сессия остаётся действующей
	_, err = ts.repo.FetchOneByAccessToken(ts.ctx, "access-a-2")
	ts.Require().NoError(err)

	err = ts.repo.RevokeTokensBySessionID(ts.ctx, userID, 1)
	ts.Require().ErrorIs(err, errors.ErrEventStorageRecordsNotAffected)
}

// Test_DeleteExpiredNonRefresh - истёкшие не-refresh токены удаляются пачками не более limit,
// начиная с самых старых; живые токены и истёкшие refresh токены не трогаются.
func (ts *AuthTokenPostgresTestSuite) Test_DeleteExpiredNonRefresh() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/DeleteExpiredNonRefresh")

	count, err := ts.repo.DeleteExpiredNonRefresh(ts.ctx, 2)
	ts.Require().NoError(err)
	ts.Equal(2, count)
	ts.Equal([]string{"access-live", "access-mid", "refresh-expired"}, ts.fetchTokens())

	count, err = ts.repo.DeleteExpiredNonRefresh(ts.ctx, 2)
	ts.Require().NoError(err)
	ts.Equal(1, count)

	count, err = ts.repo.DeleteExpiredNonRefresh(ts.ctx, 2)
	ts.Require().NoError(err)
	ts.Equal(0, count)
	ts.Equal([]string{"access-live", "refresh-expired"}, ts.fetchTokens())
}

// fetchTokens - значения всех токенов таблицы в алфавитном порядке.
func (ts *AuthTokenPostgresTestSuite) fetchTokens() []string {
	rows, err := ts.pgt.ConnManager().Conn(ts.ctx).Query(
		ts.ctx,
		`SELECT auth_token FROM `+authTokensTableName+` ORDER BY auth_token;`,
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

// Test_FetchOneByAccessToken - по действующему access токену возвращается область действия его сессии.
func (ts *AuthTokenPostgresTestSuite) Test_FetchOneByAccessToken() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/FetchOneByAccessToken")

	row, err := ts.repo.FetchOneByAccessToken(ts.ctx, "access-live")
	ts.Require().NoError(err)
	ts.Equal(
		dto.UserScopes{
			UserID:    uuid.MustParse(fixtureUserA),
			SessionID: 1,
			Realm:     "app/users",
			Kind:      "regular",
			LangCode:  "ru-RU",
			TimeZone:  "Europe/Moscow",
		},
		row,
	)
}

// Test_FetchOneByAccessTokenWhenExpired - истёкший, но не отозванный токен возвращает
// mrauth.ErrEventTokenExpired.
func (ts *AuthTokenPostgresTestSuite) Test_FetchOneByAccessTokenWhenExpired() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/FetchOneByAccessToken")

	_, err := ts.repo.FetchOneByAccessToken(ts.ctx, "access-expired")
	ts.Require().ErrorIs(err, mrauth.ErrEventTokenExpired)
}

// Test_FetchOneByAccessTokenWhenRevoked - отозванный токен не находится: ErrEventStorageNoRecordFound.
func (ts *AuthTokenPostgresTestSuite) Test_FetchOneByAccessTokenWhenRevoked() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/FetchOneByAccessToken")

	_, err := ts.repo.FetchOneByAccessToken(ts.ctx, "access-revoked")
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_FetchOneByAccessTokenWhenNotAccessToken - токен другого типа, как и неизвестный,
// не находится: ErrEventStorageNoRecordFound.
func (ts *AuthTokenPostgresTestSuite) Test_FetchOneByAccessTokenWhenNotAccessToken() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/FetchOneByAccessToken")

	_, err := ts.repo.FetchOneByAccessToken(ts.ctx, "refresh-live")
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)

	_, err = ts.repo.FetchOneByAccessToken(ts.ctx, "access-unknown")
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_FetchOpenSessionCount - считаются различные сессии пользователя в указанном realm, у которых
// есть действующий не истёкший refresh токен.
func (ts *AuthTokenPostgresTestSuite) Test_FetchOpenSessionCount() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/FetchOpenSessionCount")

	userID := uuid.MustParse(fixtureUserA)

	count, err := ts.repo.FetchOpenSessionCount(ts.ctx, userID, realmA)
	ts.Require().NoError(err)
	ts.Equal(2, count)

	count, err = ts.repo.FetchOpenSessionCount(ts.ctx, userID, realmB)
	ts.Require().NoError(err)
	ts.Equal(1, count)
}

// Test_FetchOpenSessionCountWhenNoSessions - у пользователя без открытых сессий - ноль, а не ошибка.
func (ts *AuthTokenPostgresTestSuite) Test_FetchOpenSessionCountWhenNoSessions() {
	count, err := ts.repo.FetchOpenSessionCount(ts.ctx, uuid.MustParse(fixtureUserA), realmA)
	ts.Require().NoError(err)
	ts.Equal(0, count)
}

// Test_FetchLastEnabledPairBySessionID - возвращаются действующий refresh токен сессии
// и последний выпущенный действующий access токен; отозванные токены не учитываются.
func (ts *AuthTokenPostgresTestSuite) Test_FetchLastEnabledPairBySessionID() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/FetchLastEnabledPairBySessionID")

	userID := uuid.MustParse(fixtureUserA)
	scopes := entity.AuthTokenScopes{Realm: "app/users", UserKind: "regular", LangCode: "ru-RU", TimeZone: "Europe/Moscow"}

	access, refresh, err := ts.repo.FetchLastEnabledPairBySessionID(ts.ctx, userID, 1)
	ts.Require().NoError(err)
	ts.Equal(
		entity.AuthToken{
			Token:     "access-new",
			Type:      authtokentype.Access,
			UserID:    userID,
			SessionID: 1,
			Scopes:    scopes,
			ExpiresAt: time.Date(2099, 1, 2, 0, 0, 0, 0, time.UTC),
		},
		access,
	)
	ts.Equal(
		entity.AuthToken{
			Token:     "refresh-a-1",
			Type:      authtokentype.Refresh,
			UserID:    userID,
			SessionID: 1,
			Scopes:    scopes,
			ExpiresAt: time.Date(2099, 2, 1, 0, 0, 0, 0, time.UTC),
		},
		refresh,
	)
}

// Test_FetchLastEnabledPairBySessionIDWhenNoAccessToken - у сессии без хранимого access токена (JWT)
// возвращается только refresh токен, access - пустой.
func (ts *AuthTokenPostgresTestSuite) Test_FetchLastEnabledPairBySessionIDWhenNoAccessToken() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/FetchLastEnabledPairBySessionID")

	access, refresh, err := ts.repo.FetchLastEnabledPairBySessionID(ts.ctx, uuid.MustParse(fixtureUserA), 2)
	ts.Require().NoError(err)
	ts.Equal(entity.AuthToken{}, access)
	ts.Equal("refresh-a-2", refresh.Token)
}

// Test_FetchLastEnabledPairBySessionIDWhenRefreshExpired - истёкший действующий refresh токен
// не выдаётся: возвращается mrauth.ErrEventTokenExpired.
func (ts *AuthTokenPostgresTestSuite) Test_FetchLastEnabledPairBySessionIDWhenRefreshExpired() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/FetchLastEnabledPairBySessionID")

	_, _, err := ts.repo.FetchLastEnabledPairBySessionID(ts.ctx, uuid.MustParse(fixtureUserA), 3)
	ts.Require().ErrorIs(err, mrauth.ErrEventTokenExpired)
}

// Test_FetchLastEnabledPairBySessionIDWhenNotExists - у сессии без действующего refresh токена
// возвращается ErrEventStorageNoRecordFound.
func (ts *AuthTokenPostgresTestSuite) Test_FetchLastEnabledPairBySessionIDWhenNotExists() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/FetchLastEnabledPairBySessionID")

	_, _, err := ts.repo.FetchLastEnabledPairBySessionID(ts.ctx, uuid.MustParse(fixtureUserA), 99)
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_DeleteExpiredRefresh - истёкшие refresh токены (в любом статусе) удаляются пачками не более
// limit, начиная с самых старых, и возвращаются их сессии - по одной записи на удалённый токен;
// живые токены и истёкшие токены других типов не трогаются.
func (ts *AuthTokenPostgresTestSuite) Test_DeleteExpiredRefresh() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/AuthToken/DeleteExpiredRefresh")

	userA := uuid.MustParse(fixtureUserA)

	candidates, err := ts.repo.DeleteExpiredRefresh(ts.ctx, 3)
	ts.Require().NoError(err)
	ts.ElementsMatch(
		[]entity.SessionPK{
			{UserID: userA, SessionID: 1},
			{UserID: userA, SessionID: 2},
			{UserID: userA, SessionID: 2},
		},
		candidates,
	)

	candidates, err = ts.repo.DeleteExpiredRefresh(ts.ctx, 3)
	ts.Require().NoError(err)
	ts.Equal([]entity.SessionPK{{UserID: uuid.MustParse(fixtureUserB), SessionID: 3}}, candidates)

	candidates, err = ts.repo.DeleteExpiredRefresh(ts.ctx, 3)
	ts.Require().NoError(err)
	ts.Empty(candidates)
	ts.Equal([]string{"access-expired", "refresh-live"}, ts.fetchTokens())
}
