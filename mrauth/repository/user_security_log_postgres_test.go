package repository_test

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/mrtype"
	"github.com/mondegor/go-storage/mrtests/pgtest"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/securityevent"
	"github.com/mondegor/go-components/mrauth/repository"
	"github.com/mondegor/go-components/tests"
)

const usersSecurityLogTableName = "sample_schema.users_security_log"

type UserSecurityLogPostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *pgtest.Tester
	repo *repository.UserSecurityLogPostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestUserSecurityLogPostgresTestSuite(t *testing.T) {
	suite.Run(t, new(UserSecurityLogPostgresTestSuite))
}

func (ts *UserSecurityLogPostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrauth"))
	ts.repo = repository.NewUserSecurityLogPostgres(ts.pgt.ConnManager(), usersSecurityLogTableName)
}

func (ts *UserSecurityLogPostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// fetchAll - все записи пользователя одной страницей.
func (ts *UserSecurityLogPostgresTestSuite) fetchAll(userID uuid.UUID) []entity.SecurityLogEvent {
	rows, hasNext, err := ts.repo.FetchByUserID(ts.ctx, userID, mrstorage.IDCursor{Limit: 100})
	ts.Require().NoError(err)
	ts.Require().False(hasNext)

	return rows
}

// Test_Insert - запись с подробностями и прокси-адресом, а также запись без подробностей,
// без прокси и с пустым user agent читаются без искажений; время события ставит хранилище при записи.
func (ts *UserSecurityLogPostgresTestSuite) Test_Insert() {
	userID := uuid.MustParse(fixtureUserA)
	remaining := 7

	full := entity.SecurityLogEvent{
		UserID:    userID,
		EventType: securityevent.EmailChanged,
		ClientIP:  mrtype.NewDetailedIP(netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("10.0.0.1")),
		UserAgent: "Mozilla/5.0",
		Extra: &entity.SecurityLogExtra{
			OldValue:  "old@example.com",
			NewValue:  "new@example.com",
			Factor:    "TOTP",
			Remaining: &remaining,
		},
	}

	bare := entity.SecurityLogEvent{
		UserID:    userID,
		EventType: securityevent.Auth2FADisabled,
		ClientIP:  mrtype.NewIP(netip.MustParseAddr("2001:db8::1")),
	}

	ts.Require().NoError(ts.repo.Insert(ts.ctx, full))
	ts.Require().NoError(ts.repo.Insert(ts.ctx, bare))

	got := ts.fetchAll(userID)
	ts.Require().Len(got, 2)

	// от свежей к старой
	ts.Greater(got[0].RecordID, got[1].RecordID)

	ts.Equal(userID, got[0].UserID)
	ts.Equal(securityevent.Auth2FADisabled, got[0].EventType)
	ts.Equal(netip.MustParseAddr("2001:db8::1"), got[0].ClientIP.Real)
	ts.False(got[0].ClientIP.Proxy.IsValid())
	ts.Empty(got[0].UserAgent)
	ts.Nil(got[0].Extra)
	ts.WithinDuration(time.Now(), got[0].CreatedAt, time.Minute)

	ts.Equal(securityevent.EmailChanged, got[1].EventType)
	ts.Equal(netip.MustParseAddr("127.0.0.1"), got[1].ClientIP.Real)
	ts.Equal(netip.MustParseAddr("10.0.0.1"), got[1].ClientIP.Proxy)
	ts.Equal("Mozilla/5.0", got[1].UserAgent)
	ts.Equal(full.Extra, got[1].Extra)
	ts.Equal(time.UTC, got[1].CreatedAt.Location())
}

// Test_InsertWhenClientIPUnset - незаданный real IP отвергается ограничением NOT NULL.
func (ts *UserSecurityLogPostgresTestSuite) Test_InsertWhenClientIPUnset() {
	userID := uuid.MustParse(fixtureUserA)

	ts.Require().Error(ts.repo.Insert(ts.ctx, entity.NewSecurityLogEvent(
		userID, mrtype.DetailedIP{}, "", securityevent.SignedIn, nil,
	)))
	ts.Empty(ts.fetchAll(userID))
}

// Test_FetchByUserID - курсор листает журнал страницами от свежих записей к старым,
// признак продолжения выставляется, пока за страницей есть записи, хвост пуст.
func (ts *UserSecurityLogPostgresTestSuite) Test_FetchByUserID() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/UserSecurityLog/FetchByUserID")

	userID := uuid.MustParse(fixtureUserA)

	all := ts.fetchAll(userID)
	ts.Require().Len(all, 5)

	page1, hasNext, err := ts.repo.FetchByUserID(ts.ctx, userID, mrstorage.IDCursor{Limit: 2})
	ts.Require().NoError(err)
	ts.True(hasNext)
	ts.Equal(all[:2], page1)

	page2, hasNext, err := ts.repo.FetchByUserID(ts.ctx, userID, mrstorage.IDCursor{ID: page1[1].RecordID, Limit: 2})
	ts.Require().NoError(err)
	ts.True(hasNext)
	ts.Equal(all[2:4], page2)

	page3, hasNext, err := ts.repo.FetchByUserID(ts.ctx, userID, mrstorage.IDCursor{ID: page2[1].RecordID, Limit: 2})
	ts.Require().NoError(err)
	ts.False(hasNext)
	ts.Equal(all[4:], page3)

	tail, hasNext, err := ts.repo.FetchByUserID(ts.ctx, userID, mrstorage.IDCursor{ID: page3[0].RecordID, Limit: 2})
	ts.Require().NoError(err)
	ts.False(hasNext)
	ts.Empty(tail)
}

// Test_FetchByUserIDWhenOtherUsers - выборка не захватывает записи других пользователей.
func (ts *UserSecurityLogPostgresTestSuite) Test_FetchByUserIDWhenOtherUsers() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/UserSecurityLog/FetchByUserIDWhenOtherUsers")

	userID := uuid.MustParse(fixtureUserA)

	got := ts.fetchAll(userID)
	ts.Require().Len(got, 2)

	for _, row := range got {
		ts.Equal(userID, row.UserID)
	}
}

// Test_DeleteBeforeDate - удаляются только записи старше границы и не более limit за вызов.
func (ts *UserSecurityLogPostgresTestSuite) Test_DeleteBeforeDate() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/UserSecurityLog/DeleteBeforeDate")

	userID := uuid.MustParse(fixtureUserA)

	border := time.Now().Add(-24 * time.Hour)

	count, err := ts.repo.DeleteBeforeDate(ts.ctx, border, 2)
	ts.Require().NoError(err)
	ts.Equal(2, count)

	count, err = ts.repo.DeleteBeforeDate(ts.ctx, border, 2)
	ts.Require().NoError(err)
	ts.Equal(1, count)

	got := ts.fetchAll(userID)
	ts.Require().Len(got, 1)
	ts.True(got[0].CreatedAt.After(border))
}
