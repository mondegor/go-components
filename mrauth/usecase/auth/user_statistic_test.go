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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/usecase/auth"
)

type fakeStatUpdater struct {
	err      error
	captured []dto.UserActivityLastVisited
}

func (f *fakeStatUpdater) UpdateLastVisited(_ context.Context, rows []dto.UserActivityLastVisited) error {
	f.captured = rows

	return f.err
}

type fakeLogStorage struct {
	err      error
	captured []dto.UserActivityLogMessage
}

func (f *fakeLogStorage) Insert(_ context.Context, rows []dto.UserActivityLogMessage) error {
	f.captured = rows

	return f.err
}

type fakeSessionUpdater struct {
	err      error
	captured []dto.SessionLastActivity
}

func (f *fakeSessionUpdater) UpdateLastActivity(_ context.Context, rows []dto.SessionLastActivity) error {
	f.captured = rows

	return f.err
}

// warnCountingLogger - считает предупреждения; остальные методы остаются no-op от вложенного логгера.
type warnCountingLogger struct {
	mrlog.Logger

	warns int
}

func (l *warnCountingLogger) Warn(context.Context, string, ...any) {
	l.warns++
}

func findSession(rows []dto.SessionLastActivity, userID uuid.UUID, sessionID uint32) (dto.SessionLastActivity, bool) {
	for _, row := range rows {
		if row.UserID == userID && row.SessionID == sessionID {
			return row, true
		}
	}

	return dto.SessionLastActivity{}, false
}

func TestUserStatistic_Execute_SessionsLastActivity(t *testing.T) {
	t.Parallel()

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

	sessionUpdater := &fakeSessionUpdater{}
	statUpdater := &fakeStatUpdater{}
	uc := auth.NewUserStatistic(statUpdater, &fakeLogStorage{}, sessionUpdater, mrlog.NopLogger())

	require.NoError(t, uc.Execute(context.Background(), messages))

	assert.Len(t, sessionUpdater.captured, 3) // (A,10), (A,11), (B,10)

	a10, ok := findSession(sessionUpdater.captured, userA, 10)
	require.True(t, ok)
	assert.Equal(t, netip.MustParseAddr("10.11.12.13"), a10.LastIP)
	assert.Equal(t, base.Add(time.Minute), a10.LastVisitedAt)

	a11, ok := findSession(sessionUpdater.captured, userA, 11)
	require.True(t, ok)
	assert.Equal(t, netip.MustParseAddr("5.6.7.8"), a11.LastIP)

	b10, ok := findSession(sessionUpdater.captured, userB, 10)
	require.True(t, ok)
	assert.Equal(t, netip.MustParseAddr("9.10.11.12"), b10.LastIP)

	_, ok = findSession(sessionUpdater.captured, userA, 0)
	assert.False(t, ok)

	// статистика последнего посещения агрегируется по пользователям (A и B)
	assert.Len(t, statUpdater.captured, 2)
}

func TestUserStatistic_Execute_StatPerRealm(t *testing.T) {
	t.Parallel()

	user := uuid.New()
	base := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)

	messages := []dto.UserActivityLogMessage{
		{UserID: user, RealmID: 1, SessionID: 1, UserIP: mrtype.NewIP(netip.MustParseAddr("1.2.3.4")), VisitedAt: base},
		// тот же пользователь и realm, более позднее посещение - должно победить
		{UserID: user, RealmID: 1, SessionID: 1, UserIP: mrtype.NewIP(netip.MustParseAddr("1.2.3.4")), VisitedAt: base.Add(time.Minute)},
		// тот же пользователь, другой realm - отдельная строка статистики
		{UserID: user, RealmID: 2, SessionID: 2, UserIP: mrtype.NewIP(netip.MustParseAddr("5.6.7.8")), VisitedAt: base.Add(time.Hour)},
	}

	statUpdater := &fakeStatUpdater{}
	uc := auth.NewUserStatistic(statUpdater, &fakeLogStorage{}, &fakeSessionUpdater{}, mrlog.NopLogger())

	require.NoError(t, uc.Execute(context.Background(), messages))

	// одна строка на пару (user, realm): realm 1 и realm 2
	require.Len(t, statUpdater.captured, 2)

	got := make(map[uint16]dto.UserActivityLastVisited, len(statUpdater.captured))

	for _, row := range statUpdater.captured {
		assert.Equal(t, user, row.UserID)
		got[row.RealmID] = row
	}

	require.Contains(t, got, uint16(1))
	require.Contains(t, got, uint16(2))
	assert.Equal(t, base.Add(time.Minute), got[1].LastVisitedAt) // позднее посещение realm 1
	assert.Equal(t, base.Add(time.Hour), got[2].LastVisitedAt)
}

// TestUserStatistic_Execute_UnknownRealmSentinel - сообщение с сентинелом RealmID = 0
// (realm не определён, см. dto.UserActivityLogMessage) обновляет сессию и попадает в журнал,
// но не порождает строку per-realm статистики - её для realm 0 не существует.
func TestUserStatistic_Execute_UnknownRealmSentinel(t *testing.T) {
	t.Parallel()

	user := uuid.New()
	base := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)

	messages := []dto.UserActivityLogMessage{
		{UserID: user, RealmID: 0, SessionID: 7, UserIP: mrtype.NewIP(netip.MustParseAddr("1.2.3.4")), VisitedAt: base},
	}

	statUpdater := &fakeStatUpdater{}
	logStorage := &fakeLogStorage{}
	sessionUpdater := &fakeSessionUpdater{}
	uc := auth.NewUserStatistic(statUpdater, logStorage, sessionUpdater, mrlog.NopLogger())

	require.NoError(t, uc.Execute(context.Background(), messages))

	// keep-alive сессии не должен зависеть от realm'а
	require.Len(t, sessionUpdater.captured, 1)
	assert.Equal(t, uint32(7), sessionUpdater.captured[0].SessionID)
	assert.Equal(t, base, sessionUpdater.captured[0].LastVisitedAt)

	require.Len(t, logStorage.captured, 1)
	assert.Equal(t, uint16(0), logStorage.captured[0].RealmID)

	assert.Empty(t, statUpdater.captured)
}

// TestUserStatistic_Execute_StatTotalMissLogsWarning - пакет, ни одна пара (user, realm)
// которого не имеет строки статистики (ErrEventStorageRecordsNotAffected), не проваливается
// (иначе он бесконечно ретраился бы), но деградация сигналится предупреждением в лог,
// а журнал активности вставляется как обычно.
func TestUserStatistic_Execute_StatTotalMissLogsWarning(t *testing.T) {
	t.Parallel()

	logger := &warnCountingLogger{Logger: mrlog.NopLogger()}
	statUpdater := &fakeStatUpdater{err: sysmesserrors.ErrEventStorageRecordsNotAffected}
	logStorage := &fakeLogStorage{}
	uc := auth.NewUserStatistic(statUpdater, logStorage, &fakeSessionUpdater{}, logger)

	messages := []dto.UserActivityLogMessage{
		{UserID: uuid.New(), RealmID: 1, SessionID: 1, UserIP: mrtype.NewIP(netip.MustParseAddr("1.2.3.4")), VisitedAt: time.Now()},
	}

	require.NoError(t, uc.Execute(context.Background(), messages))
	assert.Equal(t, 1, logger.warns)
	assert.Len(t, logStorage.captured, 1) // журнал не зависит от промаха статистики
}

func TestUserStatistic_Execute_Empty(t *testing.T) {
	t.Parallel()

	sessionUpdater := &fakeSessionUpdater{}
	uc := auth.NewUserStatistic(&fakeStatUpdater{}, &fakeLogStorage{}, sessionUpdater, mrlog.NopLogger())

	require.NoError(t, uc.Execute(context.Background(), nil))
	assert.Empty(t, sessionUpdater.captured)
}

func TestUserStatistic_Execute_StorageErrors(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("boom")

	tests := []struct {
		name    string
		stat    *fakeStatUpdater
		log     *fakeLogStorage
		session *fakeSessionUpdater
	}{
		{
			name:    "session storage error",
			stat:    &fakeStatUpdater{},
			log:     &fakeLogStorage{},
			session: &fakeSessionUpdater{err: errBoom},
		},
		{
			name:    "stat storage error",
			stat:    &fakeStatUpdater{err: errBoom},
			log:     &fakeLogStorage{},
			session: &fakeSessionUpdater{},
		},
		{
			name:    "log storage error",
			stat:    &fakeStatUpdater{},
			log:     &fakeLogStorage{err: errBoom},
			session: &fakeSessionUpdater{},
		},
	}

	messages := []dto.UserActivityLogMessage{
		{UserID: uuid.New(), SessionID: 1, UserIP: mrtype.NewIP(netip.MustParseAddr("1.2.3.4")), VisitedAt: time.Now()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := auth.NewUserStatistic(tt.stat, tt.log, tt.session, mrlog.NopLogger())

			require.Error(t, uc.Execute(context.Background(), messages))
		})
	}
}
