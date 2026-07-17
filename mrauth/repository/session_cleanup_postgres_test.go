package repository_test

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-storage/mrtests/infra"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/tests"
)

const (
	sessionsTableName             = "sample_schema.sessions"
	authTokensTableName           = "sample_schema.auth_tokens"
	sessionsCleanupQueueTableName = "sample_schema.sessions_cleanup_queue"

	// значения enum'ов из миграции (token_type: 2=REFRESH; token_status: 1=ENABLED, 2=REVOKED).
	tokenTypeRefresh   = 2
	tokenStatusEnabled = 1
	tokenStatusRevoked = 2
)

type SessionCleanupPostgresTestSuite struct {
	suite.Suite

	ctx context.Context
	pgt *infra.PostgresTester
}

func TestSessionCleanupPostgresTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SessionCleanupPostgresTestSuite))
}

func (ts *SessionCleanupPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = infra.NewPostgresTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(tests.AppWorkDir() + "/mrauth/_sample/migrations")
}

func (ts *SessionCleanupPostgresTestSuite) TearDownSuite() {
	ts.pgt.Destroy(ts.ctx)
}

func (ts *SessionCleanupPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.ctx)
}

func (ts *SessionCleanupPostgresTestSuite) seedSession(userID uuid.UUID, sessionID uint32) {
	err := ts.pgt.ConnManager().Conn(ts.ctx).Exec(
		ts.ctx,
		// last_ip обязателен (NOT NULL); для очереди очистки его значение не важно
		`INSERT INTO sample_schema.sessions (user_id, session_id, last_ip) VALUES ($1, $2, '127.0.0.1');`,
		userID,
		sessionID,
	)
	ts.Require().NoError(err)
}

func (ts *SessionCleanupPostgresTestSuite) seedSessionAt(userID uuid.UUID, sessionID uint32, updatedAt time.Time) {
	err := ts.pgt.ConnManager().Conn(ts.ctx).Exec(
		ts.ctx,
		// last_ip обязателен (NOT NULL); для очереди очистки его значение не важно
		`INSERT INTO sample_schema.sessions (user_id, session_id, last_ip, updated_at) VALUES ($1, $2, '127.0.0.1', $3);`,
		userID,
		sessionID,
		updatedAt,
	)
	ts.Require().NoError(err)
}

func (ts *SessionCleanupPostgresTestSuite) seedToken(userID uuid.UUID, sessionID uint32, tokenType, tokenStatus int, expiresAt time.Time) {
	err := ts.pgt.ConnManager().Conn(ts.ctx).Exec(
		ts.ctx,
		`INSERT INTO sample_schema.auth_tokens
			(auth_token, token_type, user_id, realm_id, session_id, token_scopes, token_status, expires_at)
		VALUES
			($1, $2, $3, 1, $4, '{}'::jsonb, $5, $6);`,
		uuid.NewString(),
		tokenType,
		userID,
		sessionID,
		tokenStatus,
		expiresAt,
	)
	ts.Require().NoError(err)
}

// TestDeleteOrphaned - удаляются только осиротевшие сессии (без живого ENABLED непросроченного
// refresh-токена); живая сессия (в т.ч. после ротации) сохраняется.
func (ts *SessionCleanupPostgresTestSuite) TestDeleteOrphaned() {
	userID := uuid.New()
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)

	ts.seedSession(userID, 1) // нет токенов -> осиротевшая
	ts.seedSession(userID, 2) // живой ENABLED refresh -> НЕ осиротевшая
	ts.seedToken(userID, 2, tokenTypeRefresh, tokenStatusEnabled, future)
	ts.seedSession(userID, 3) // только REVOKED refresh -> осиротевшая
	ts.seedToken(userID, 3, tokenTypeRefresh, tokenStatusRevoked, future)
	ts.seedSession(userID, 4) // истёкший ENABLED refresh -> осиротевшая
	ts.seedToken(userID, 4, tokenTypeRefresh, tokenStatusEnabled, past)

	candidates := []entity.SessionPK{
		{UserID: userID, SessionID: 1},
		{UserID: userID, SessionID: 2},
		{UserID: userID, SessionID: 3},
		{UserID: userID, SessionID: 4},
	}

	repo := repository.NewOrphanSessionDeleterPostgres(ts.pgt.ConnManager(), sessionsTableName, authTokensTableName)
	ts.Require().NoError(repo.DeleteOrphaned(ts.ctx, candidates))

	sessionRepo := repository.NewSessionPostgres(ts.pgt.ConnManager(), sessionsTableName)

	rows, err := sessionRepo.FetchOrderedListByUserIDAndSessionIDs(ts.ctx, userID, []uint32{1, 2, 3, 4}, 0)
	ts.Require().NoError(err)
	ts.Require().Len(rows, 1)
	ts.Equal(uint32(2), rows[0].SessionID) // осталась только живая сессия
}

// TestInsertSessionIDCollision - повторная вставка той же пары (user_id, session_id)
// возвращает ErrEventRecordAlreadyExists, а не дублирует строку.
func (ts *SessionCleanupPostgresTestSuite) TestInsertSessionIDCollision() {
	userID := uuid.New()
	repo := repository.NewSessionPostgres(ts.pgt.ConnManager(), sessionsTableName)

	row := entity.Session{UserID: userID, SessionID: 42, UserAgent: "ua", LastIP: netip.MustParseAddr("127.0.0.1")}

	ts.Require().NoError(repo.Insert(ts.ctx, row))

	err := repo.Insert(ts.ctx, row)
	ts.Require().ErrorIs(err, errors.ErrEventRecordAlreadyExists)
}

// TestFetchOrderedListByUserIDAndSessionIDs - выборка упорядочена активными вперёд (updated_at DESC,
// при равенстве - больший session_id вперёд), а положительный limit оставляет только новейшие.
func (ts *SessionCleanupPostgresTestSuite) TestFetchOrderedListByUserIDAndSessionIDs() {
	userID := uuid.New()
	base := time.Now()

	// строки сеются в перемешанном порядке относительно ожидаемого результата
	ts.seedSessionAt(userID, 1, base.Add(-3*time.Minute)) // наименее активная
	ts.seedSessionAt(userID, 3, base.Add(-1*time.Minute)) // наиболее активная
	ts.seedSessionAt(userID, 2, base.Add(-2*time.Minute))

	sessionRepo := repository.NewSessionPostgres(ts.pgt.ConnManager(), sessionsTableName)

	// без лимита: все три, новыми вперёд
	rows, err := sessionRepo.FetchOrderedListByUserIDAndSessionIDs(ts.ctx, userID, []uint32{1, 2, 3}, 0)
	ts.Require().NoError(err)
	ts.Require().Len(rows, 3)
	ts.Equal([]uint32{3, 2, 1}, []uint32{rows[0].SessionID, rows[1].SessionID, rows[2].SessionID})

	// limit=2: только две новейшие
	rows, err = sessionRepo.FetchOrderedListByUserIDAndSessionIDs(ts.ctx, userID, []uint32{1, 2, 3}, 2)
	ts.Require().NoError(err)
	ts.Require().Len(rows, 2)
	ts.Equal([]uint32{3, 2}, []uint32{rows[0].SessionID, rows[1].SessionID})
}

// TestEvictExpireDrainChain - сквозная цепочка: эвикт сессии (revoke её refresh-токена) ->
// истечение и удаление refresh-токена с постановкой его сессии в очередь очистки ->
// слив очереди (удаление осиротевшей сессии). Проверяет, что три независимо планируемых
// шага вместе доводят вытесненную сессию до удаления.
func (ts *SessionCleanupPostgresTestSuite) TestEvictExpireDrainChain() {
	const sessionID uint32 = 1

	userID := uuid.New()

	// живая сессия с ENABLED refresh-токеном
	ts.seedSession(userID, sessionID)
	ts.seedToken(userID, sessionID, tokenTypeRefresh, tokenStatusEnabled, time.Now().Add(time.Hour))

	conn := ts.pgt.ConnManager()
	authRepo := repository.NewAuthTokenPostgres(conn, authTokensTableName)
	queueRepo := repository.NewSessionCleanupQueuePostgres(conn, sessionsCleanupQueueTableName)
	orphanRepo := repository.NewOrphanSessionDeleterPostgres(conn, sessionsTableName, authTokensTableName)
	sessionRepo := repository.NewSessionPostgres(conn, sessionsTableName)

	// 1. эвикт: токен сессии переводится в REVOKED с expires_at = NOW()
	ts.Require().NoError(authRepo.RevokeTokensBySessionIDs(ts.ctx, userID, []uint32{sessionID}))

	// 2. истёкший refresh-токен удаляется, его сессия попадает в кандидаты на очистку
	candidates, err := authRepo.DeleteExpiredRefresh(ts.ctx, 100)
	ts.Require().NoError(err)
	ts.Require().Equal([]entity.SessionPK{{UserID: userID, SessionID: sessionID}}, candidates)

	// 3. постановка кандидатов в очередь и выборка пачки
	ts.Require().NoError(queueRepo.Enqueue(ts.ctx, candidates))

	pks, err := queueRepo.Fetch(ts.ctx, 100)
	ts.Require().NoError(err)
	ts.Require().Equal(candidates, pks)

	// 4. слив очереди: осиротевшая сессия удаляется, пачка подтверждается (ack)
	ts.Require().NoError(orphanRepo.DeleteOrphaned(ts.ctx, pks))
	ts.Require().NoError(queueRepo.Delete(ts.ctx, pks))

	// сессия удалена, очередь пуста
	rows, err := sessionRepo.FetchOrderedListByUserIDAndSessionIDs(ts.ctx, userID, []uint32{sessionID}, 0)
	ts.Require().NoError(err)
	ts.Empty(rows)

	remaining, err := queueRepo.Fetch(ts.ctx, 100)
	ts.Require().NoError(err)
	ts.Empty(remaining)
}
