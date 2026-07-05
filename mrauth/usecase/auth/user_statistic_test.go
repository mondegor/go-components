package auth_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
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
		{UserID: userA, SessionID: 10, UserIP: mrtype.NewDetailedIP(0x01020304, 0), VisitedAt: base},
		// более позднее посещение той же сессии - должно победить
		{UserID: userA, SessionID: 10, UserIP: mrtype.NewDetailedIP(0x0A0B0C0D, 0), VisitedAt: base.Add(time.Minute)},
		{UserID: userA, SessionID: 11, UserIP: mrtype.NewDetailedIP(0x05060708, 0), VisitedAt: base},
		{UserID: userB, SessionID: 10, UserIP: mrtype.NewDetailedIP(0x090A0B0C, 0), VisitedAt: base},
		// запрос без сессии - должен быть пропущен
		{UserID: userA, SessionID: 0, UserIP: mrtype.NewDetailedIP(0x11111111, 0), VisitedAt: base.Add(time.Hour)},
	}

	sessionUpdater := &fakeSessionUpdater{}
	statUpdater := &fakeStatUpdater{}
	uc := auth.NewUserStatistic(statUpdater, &fakeLogStorage{}, sessionUpdater)

	require.NoError(t, uc.Execute(context.Background(), messages))

	assert.Len(t, sessionUpdater.captured, 3) // (A,10), (A,11), (B,10)

	a10, ok := findSession(sessionUpdater.captured, userA, 10)
	require.True(t, ok)
	assert.Equal(t, uint32(0x0A0B0C0D), a10.LastIP)
	assert.Equal(t, base.Add(time.Minute), a10.LastVisitedAt)

	a11, ok := findSession(sessionUpdater.captured, userA, 11)
	require.True(t, ok)
	assert.Equal(t, uint32(0x05060708), a11.LastIP)

	b10, ok := findSession(sessionUpdater.captured, userB, 10)
	require.True(t, ok)
	assert.Equal(t, uint32(0x090A0B0C), b10.LastIP)

	_, ok = findSession(sessionUpdater.captured, userA, 0)
	assert.False(t, ok)

	// статистика последнего посещения агрегируется по пользователям (A и B)
	assert.Len(t, statUpdater.captured, 2)
}

func TestUserStatistic_Execute_SkipsUnparsableIP(t *testing.T) {
	t.Parallel()

	sessionUpdater := &fakeSessionUpdater{}
	uc := auth.NewUserStatistic(&fakeStatUpdater{}, &fakeLogStorage{}, sessionUpdater)

	messages := []dto.UserActivityLogMessage{
		// 3-байтовый IP не приводится к uint32 - запись сессии пропускается
		{UserID: uuid.New(), SessionID: 7, UserIP: mrtype.DetailedIP{Real: net.IP{1, 2, 3}}, VisitedAt: time.Now()},
	}

	require.NoError(t, uc.Execute(context.Background(), messages))
	assert.Empty(t, sessionUpdater.captured)
}

func TestUserStatistic_Execute_Empty(t *testing.T) {
	t.Parallel()

	sessionUpdater := &fakeSessionUpdater{}
	uc := auth.NewUserStatistic(&fakeStatUpdater{}, &fakeLogStorage{}, sessionUpdater)

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
		{UserID: uuid.New(), SessionID: 1, UserIP: mrtype.NewDetailedIP(0x01020304, 0), VisitedAt: time.Now()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := auth.NewUserStatistic(tt.stat, tt.log, tt.session)

			require.Error(t, uc.Execute(context.Background(), messages))
		})
	}
}
