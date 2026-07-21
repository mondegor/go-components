package auth_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	sysmesserrors "github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrlog"
	"github.com/mondegor/go-core/mrtype"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/usecase/auth"
	"github.com/mondegor/go-components/mrauth/usecase/auth/mock"
)

// warnCountingLogger - считает вызовы Warn; остальные методы остаются no-op от вложенного логгера.
//
// Это не мок коллаборатора, а зонд для проверки сигнала о деградации: мок mrlog.Logger
// потребовал бы AnyTimes()-заглушек на весь интерфейс логгера ради одного счётчика,
// поэтому правило "моки только через mockgen" здесь сознательно не применяется.
type warnCountingLogger struct {
	mrlog.Logger

	warns int
}

func (l *warnCountingLogger) Warn(context.Context, string, ...any) {
	l.warns++
}

type UserStatisticSuite struct {
	suite.Suite

	ctrl           *gomock.Controller
	ctx            context.Context
	statUpdater    *mock.MockuserActivityStatUpdater
	logStorage     *mock.MockuserActivityLogStorage
	sessionUpdater *mock.MocksessionLastActivityUpdater

	// строки, доехавшие до каждого из хранилищ
	statRows    []dto.UserActivityLastVisited
	logRows     []dto.UserActivityLogMessage
	sessionRows []dto.SessionLastActivity
}

func TestUserStatisticSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(UserStatisticSuite))
}

func (s *UserStatisticSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *UserStatisticSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.statUpdater = mock.NewMockuserActivityStatUpdater(s.ctrl)
	s.logStorage = mock.NewMockuserActivityLogStorage(s.ctrl)
	s.sessionUpdater = mock.NewMocksessionLastActivityUpdater(s.ctrl)
	s.statRows = nil
	s.logRows = nil
	s.sessionRows = nil
}

// expectStorages - все три хранилища запоминают полученный пакет строк и возвращают
// указанные ошибки; агрегация проверяется по запомненному, а не по числу вызовов.
// Вызывается каждым тестом явно: gomock выбирает первое подходящее ожидание, поэтому
// заданный в SetupTest дефолт перекрыть уже нельзя.
func (s *UserStatisticSuite) expectStorages(statErr, logErr, sessionErr error) {
	s.statUpdater.EXPECT().
		UpdateLastVisited(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, rows []dto.UserActivityLastVisited) error {
			s.statRows = rows

			return statErr
		}).
		AnyTimes()

	s.logStorage.EXPECT().
		Insert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, rows []dto.UserActivityLogMessage) error {
			s.logRows = rows

			return logErr
		}).
		AnyTimes()

	s.sessionUpdater.EXPECT().
		UpdateLastActivity(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, rows []dto.SessionLastActivity) error {
			s.sessionRows = rows

			return sessionErr
		}).
		AnyTimes()
}

func (s *UserStatisticSuite) newUseCase(logger mrlog.Logger) *auth.UserStatistic {
	return auth.NewUserStatistic(s.statUpdater, s.logStorage, s.sessionUpdater, logger)
}

func (s *UserStatisticSuite) findSession(userID uuid.UUID, sessionID uint32) (dto.SessionLastActivity, bool) {
	for _, row := range s.sessionRows {
		if row.UserID == userID && row.SessionID == sessionID {
			return row, true
		}
	}

	return dto.SessionLastActivity{}, false
}

func (s *UserStatisticSuite) TestExecuteSessionsLastActivity() {
	s.expectStorages(nil, nil, nil)

	userA := uuid.New()
	userB := uuid.New()
	base := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)

	messages := []dto.UserActivityLogMessage{
		{UserID: userA, RealmID: 1, SessionID: 10, UserIP: mrtype.NewIP(netip.MustParseAddr("1.2.3.4")), VisitedAt: base},
		// более позднее посещение той же сессии - должно победить
		{UserID: userA, RealmID: 1, SessionID: 10, UserIP: mrtype.NewIP(netip.MustParseAddr("10.11.12.13")), VisitedAt: base.Add(time.Minute)},
		{UserID: userA, RealmID: 1, SessionID: 11, UserIP: mrtype.NewIP(netip.MustParseAddr("5.6.7.8")), VisitedAt: base},
		{UserID: userB, RealmID: 1, SessionID: 10, UserIP: mrtype.NewIP(netip.MustParseAddr("9.10.11.12")), VisitedAt: base},
		// запрос без сессии - должен быть пропущен
		{UserID: userA, RealmID: 1, SessionID: 0, UserIP: mrtype.NewIP(netip.MustParseAddr("17.17.17.17")), VisitedAt: base.Add(time.Hour)},
	}

	s.Require().NoError(s.newUseCase(mrlog.NopLogger()).Execute(s.ctx, messages))

	s.Len(s.sessionRows, 3) // (A,10), (A,11), (B,10)

	a10, ok := s.findSession(userA, 10)
	s.Require().True(ok)
	s.Equal(netip.MustParseAddr("10.11.12.13"), a10.LastIP)
	s.Equal(base.Add(time.Minute), a10.LastVisitedAt)

	a11, ok := s.findSession(userA, 11)
	s.Require().True(ok)
	s.Equal(netip.MustParseAddr("5.6.7.8"), a11.LastIP)

	b10, ok := s.findSession(userB, 10)
	s.Require().True(ok)
	s.Equal(netip.MustParseAddr("9.10.11.12"), b10.LastIP)

	_, ok = s.findSession(userA, 0)
	s.False(ok)

	// статистика последнего посещения агрегируется по пользователям (A и B)
	s.Len(s.statRows, 2)
}

func (s *UserStatisticSuite) TestExecuteStatPerRealm() {
	s.expectStorages(nil, nil, nil)

	user := uuid.New()
	base := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)

	messages := []dto.UserActivityLogMessage{
		{UserID: user, RealmID: 1, SessionID: 1, UserIP: mrtype.NewIP(netip.MustParseAddr("1.2.3.4")), VisitedAt: base},
		// тот же пользователь и realm, более позднее посещение - должно победить
		{UserID: user, RealmID: 1, SessionID: 1, UserIP: mrtype.NewIP(netip.MustParseAddr("1.2.3.4")), VisitedAt: base.Add(time.Minute)},
		// тот же пользователь, другой realm - отдельная строка статистики
		{UserID: user, RealmID: 2, SessionID: 2, UserIP: mrtype.NewIP(netip.MustParseAddr("5.6.7.8")), VisitedAt: base.Add(time.Hour)},
	}

	s.Require().NoError(s.newUseCase(mrlog.NopLogger()).Execute(s.ctx, messages))

	// одна строка на пару (user, realm): realm 1 и realm 2
	s.Require().Len(s.statRows, 2)

	got := make(map[uint16]dto.UserActivityLastVisited, len(s.statRows))

	for _, row := range s.statRows {
		s.Equal(user, row.UserID)
		got[row.RealmID] = row
	}

	s.Require().Contains(got, uint16(1))
	s.Require().Contains(got, uint16(2))
	s.Equal(base.Add(time.Minute), got[1].LastVisitedAt) // позднее посещение realm 1
	s.Equal(base.Add(time.Hour), got[2].LastVisitedAt)
}

// TestExecuteUnknownRealmSentinel - сообщение с сентинелом RealmID = 0
// (realm не определён, см. dto.UserActivityLogMessage) обновляет сессию и попадает в журнал,
// но не порождает строку per-realm статистики - её для realm 0 не существует.
func (s *UserStatisticSuite) TestExecuteUnknownRealmSentinel() {
	s.expectStorages(nil, nil, nil)

	user := uuid.New()
	base := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)

	messages := []dto.UserActivityLogMessage{
		{UserID: user, RealmID: 0, SessionID: 7, UserIP: mrtype.NewIP(netip.MustParseAddr("1.2.3.4")), VisitedAt: base},
	}

	s.Require().NoError(s.newUseCase(mrlog.NopLogger()).Execute(s.ctx, messages))

	// keep-alive сессии не должен зависеть от realm'а
	s.Require().Len(s.sessionRows, 1)
	s.Equal(uint32(7), s.sessionRows[0].SessionID)
	s.Equal(base, s.sessionRows[0].LastVisitedAt)

	s.Require().Len(s.logRows, 1)
	s.Equal(uint16(0), s.logRows[0].RealmID)

	s.Empty(s.statRows)
}

// TestExecuteStatTotalMissLogsWarning - пакет, ни одна пара (user, realm) которого не имеет
// строки статистики (ErrEventStorageRecordsNotAffected), не проваливается (иначе он бесконечно
// ретраился бы), но деградация сигналится предупреждением в лог, а журнал активности
// вставляется как обычно.
func (s *UserStatisticSuite) TestExecuteStatTotalMissLogsWarning() {
	logger := &warnCountingLogger{Logger: mrlog.NopLogger()}

	s.expectStorages(sysmesserrors.ErrEventStorageRecordsNotAffected, nil, nil)

	messages := []dto.UserActivityLogMessage{
		{UserID: uuid.New(), RealmID: 1, SessionID: 1, UserIP: mrtype.NewIP(netip.MustParseAddr("1.2.3.4")), VisitedAt: time.Now()},
	}

	s.Require().NoError(s.newUseCase(logger).Execute(s.ctx, messages))
	s.Equal(1, logger.warns)
	s.Len(s.logRows, 1) // журнал не зависит от промаха статистики
}

func (s *UserStatisticSuite) TestExecuteEmpty() {
	s.expectStorages(nil, nil, nil)

	s.Require().NoError(s.newUseCase(mrlog.NopLogger()).Execute(s.ctx, nil))
	s.Empty(s.sessionRows)
}

func (s *UserStatisticSuite) TestExecuteStorageErrors() {
	errBoom := errors.New("boom")

	type testCase struct {
		name                        string
		statErr, logErr, sessionErr error
	}

	tests := []testCase{
		{name: "session storage error", sessionErr: errBoom},
		{name: "stat storage error", statErr: errBoom},
		{name: "log storage error", logErr: errBoom},
	}

	messages := []dto.UserActivityLogMessage{
		{UserID: uuid.New(), SessionID: 1, UserIP: mrtype.NewIP(netip.MustParseAddr("1.2.3.4")), VisitedAt: time.Now()},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.expectStorages(tt.statErr, tt.logErr, tt.sessionErr)

			s.Require().Error(s.newUseCase(mrlog.NopLogger()).Execute(s.ctx, messages))
		})
	}
}
