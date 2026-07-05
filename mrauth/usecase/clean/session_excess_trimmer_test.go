package clean_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/usecase/clean"
	"github.com/mondegor/go-components/mrauth/usecase/clean/mock"
)

const testRealmID uint16 = 1

type excessTrimmerMocks struct {
	uc          *clean.SessionExcessTrimmer
	consumer    *mock.MockSessionExcessQueueConsumer
	openFetcher *mock.MockOpenSessionFetcher
	lister      *mock.MockSessionLister
	closer      *mock.MockSessionCloser
	deleter     *mock.MockOrphanSessionDeleter
}

// newExcessTrimmerMocks - собирает триммер на моках; tx.Do выполняет замыкание синхронно.
func newExcessTrimmerMocks(ctrl *gomock.Controller) excessTrimmerMocks {
	tx := mock.NewMockDBTxManager(ctrl)
	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(runJob).AnyTimes()

	consumer := mock.NewMockSessionExcessQueueConsumer(ctrl)
	openFetcher := mock.NewMockOpenSessionFetcher(ctrl)
	lister := mock.NewMockSessionLister(ctrl)
	closer := mock.NewMockSessionCloser(ctrl)
	deleter := mock.NewMockOrphanSessionDeleter(ctrl)

	uc := clean.NewSessionExcessTrimmer(tx, consumer, openFetcher, lister, closer, deleter)

	return excessTrimmerMocks{
		uc:          uc,
		consumer:    consumer,
		openFetcher: openFetcher,
		lister:      lister,
		closer:      closer,
		deleter:     deleter,
	}
}

func TestSessionExcessTrimmer_Execute_EmptyQueue(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	m := newExcessTrimmerMocks(ctrl)

	m.consumer.EXPECT().Fetch(gomock.Any(), 100).Return([]entity.SessionExcessItem{}, nil)
	// trimUser / Delete не вызываются при пустой пачке

	count, err := m.uc.Execute(context.Background(), 100)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestSessionExcessTrimmer_Execute_FetchError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	m := newExcessTrimmerMocks(ctrl)

	m.consumer.EXPECT().Fetch(gomock.Any(), 100).Return(nil, errors.New("fetch failed"))

	_, err := m.uc.Execute(context.Background(), 100)
	require.Error(t, err)
}

// дубль одного устройства ревокается раньше старейшей, сессии других устройств сохраняются.
func TestSessionExcessTrimmer_Execute_RevokesDuplicatesAndOverLimit(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	m := newExcessTrimmerMocks(ctrl)

	userID := uuid.New()
	now := time.Now()

	// лимит 2; устройство A имеет дубль (s2), есть лишнее третье устройство C (s4)
	sessions := []entity.Session{
		{UserID: userID, SessionID: 1, UserAgent: "A", CreatedAt: now.Add(-1 * time.Minute), UpdatedAt: now.Add(-1 * time.Minute)},
		{UserID: userID, SessionID: 2, UserAgent: "A", CreatedAt: now.Add(-2 * time.Minute), UpdatedAt: now.Add(-2 * time.Minute)},
		{UserID: userID, SessionID: 3, UserAgent: "B", CreatedAt: now.Add(-3 * time.Minute), UpdatedAt: now.Add(-3 * time.Minute)},
		{UserID: userID, SessionID: 4, UserAgent: "C", CreatedAt: now.Add(-4 * time.Minute), UpdatedAt: now.Add(-4 * time.Minute)},
	}

	var gotRevoke []uint32

	var gotDeleted []entity.SessionPK

	gomock.InOrder(
		m.consumer.EXPECT().Fetch(gomock.Any(), 100).
			Return([]entity.SessionExcessItem{{UserID: userID, RealmID: testRealmID, SessionMax: 2}}, nil),
		m.openFetcher.EXPECT().FetchOpenSessionIDs(gomock.Any(), userID, testRealmID).
			Return([]uint32{1, 2, 3, 4}, nil),
		m.lister.EXPECT().FetchOrderedListByUserIDAndSessionIDs(gomock.Any(), userID, []uint32{1, 2, 3, 4}, 0).Return(sessions, nil),
		m.closer.EXPECT().RevokeTokensBySessionIDs(gomock.Any(), userID, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uuid.UUID, ids []uint32) error {
				gotRevoke = ids

				return nil
			}),
		m.deleter.EXPECT().DeleteOrphaned(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, pks []entity.SessionPK) error {
				gotDeleted = pks

				return nil
			}),
		m.consumer.EXPECT().Delete(gomock.Any(), []entity.SessionExcessPK{{UserID: userID, RealmID: testRealmID}}).Return(nil),
	)

	count, err := m.uc.Execute(context.Background(), 100)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// s2 - дубль устройства A, s4 - сверх лимита; s1 (A, самая активная) и s3 (B) сохранены
	require.ElementsMatch(t, []uint32{2, 4}, gotRevoke)
	require.ElementsMatch(t, []entity.SessionPK{
		{UserID: userID, SessionID: 2},
		{UserID: userID, SessionID: 4},
	}, gotDeleted)
}

// пустой user_agent не дедуплицируется (считается отдельным устройством).
func TestSessionExcessTrimmer_Execute_EmptyUserAgentNotDeduped(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	m := newExcessTrimmerMocks(ctrl)

	userID := uuid.New()
	now := time.Now()

	sessions := []entity.Session{
		{UserID: userID, SessionID: 1, UserAgent: "", CreatedAt: now.Add(-1 * time.Minute), UpdatedAt: now.Add(-1 * time.Minute)},
		{UserID: userID, SessionID: 2, UserAgent: "", CreatedAt: now.Add(-2 * time.Minute), UpdatedAt: now.Add(-2 * time.Minute)},
	}

	var gotRevoke []uint32

	gomock.InOrder(
		m.consumer.EXPECT().Fetch(gomock.Any(), 100).
			Return([]entity.SessionExcessItem{{UserID: userID, RealmID: testRealmID, SessionMax: 1}}, nil),
		m.openFetcher.EXPECT().FetchOpenSessionIDs(gomock.Any(), userID, testRealmID).Return([]uint32{1, 2}, nil),
		m.lister.EXPECT().FetchOrderedListByUserIDAndSessionIDs(gomock.Any(), userID, []uint32{1, 2}, 0).Return(sessions, nil),
		m.closer.EXPECT().RevokeTokensBySessionIDs(gomock.Any(), userID, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uuid.UUID, ids []uint32) error {
				gotRevoke = ids

				return nil
			}),
		m.deleter.EXPECT().DeleteOrphaned(gomock.Any(), gomock.Any()).Return(nil),
		m.consumer.EXPECT().Delete(gomock.Any(), []entity.SessionExcessPK{{UserID: userID, RealmID: testRealmID}}).Return(nil),
	)

	count, err := m.uc.Execute(context.Background(), 100)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// лимит 1: новейшая (s1) сохранена, более старая (s2) ревокнута как сверхлимитная, не как дубль
	require.Equal(t, []uint32{2}, gotRevoke)
}

// под лимитом и без дублей - ничего не ревокается, но пользователь снимается с очереди (ack).
func TestSessionExcessTrimmer_Execute_NothingToRevokeStillAcks(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	m := newExcessTrimmerMocks(ctrl)

	userID := uuid.New()
	now := time.Now()

	sessions := []entity.Session{
		{UserID: userID, SessionID: 1, UserAgent: "A", CreatedAt: now.Add(-1 * time.Minute)},
		{UserID: userID, SessionID: 2, UserAgent: "B", CreatedAt: now.Add(-2 * time.Minute)},
	}

	gomock.InOrder(
		m.consumer.EXPECT().Fetch(gomock.Any(), 100).
			Return([]entity.SessionExcessItem{{UserID: userID, RealmID: testRealmID, SessionMax: 4}}, nil),
		m.openFetcher.EXPECT().FetchOpenSessionIDs(gomock.Any(), userID, testRealmID).Return([]uint32{1, 2}, nil),
		m.lister.EXPECT().FetchOrderedListByUserIDAndSessionIDs(gomock.Any(), userID, []uint32{1, 2}, 0).Return(sessions, nil),
		m.consumer.EXPECT().Delete(gomock.Any(), []entity.SessionExcessPK{{UserID: userID, RealmID: testRealmID}}).Return(nil),
	)
	// Revoke / DeleteOrphaned не вызываются

	count, err := m.uc.Execute(context.Background(), 100)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

// дубли устройства В ПРЕДЕЛАХ лимита не ревокаются: дедуп срабатывает только при превышении лимита.
func TestSessionExcessTrimmer_Execute_DuplicatesUnderLimitNotRevoked(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	m := newExcessTrimmerMocks(ctrl)

	userID := uuid.New()
	now := time.Now()

	// лимит 4; у устройства A есть дубль (s2), но всего 3 сессии <= лимита - ничего не трогаем
	sessions := []entity.Session{
		{UserID: userID, SessionID: 1, UserAgent: "A", CreatedAt: now.Add(-1 * time.Minute)},
		{UserID: userID, SessionID: 2, UserAgent: "A", CreatedAt: now.Add(-2 * time.Minute)},
		{UserID: userID, SessionID: 3, UserAgent: "B", CreatedAt: now.Add(-3 * time.Minute)},
	}

	gomock.InOrder(
		m.consumer.EXPECT().Fetch(gomock.Any(), 100).
			Return([]entity.SessionExcessItem{{UserID: userID, RealmID: testRealmID, SessionMax: 4}}, nil),
		m.openFetcher.EXPECT().FetchOpenSessionIDs(gomock.Any(), userID, testRealmID).Return([]uint32{1, 2, 3}, nil),
		m.lister.EXPECT().FetchOrderedListByUserIDAndSessionIDs(gomock.Any(), userID, []uint32{1, 2, 3}, 0).Return(sessions, nil),
		m.consumer.EXPECT().Delete(gomock.Any(), []entity.SessionExcessPK{{UserID: userID, RealmID: testRealmID}}).Return(nil),
	)
	// Revoke / DeleteOrphaned не вызываются: дубль s2 сохранён, т.к. лимит не превышен

	count, err := m.uc.Execute(context.Background(), 100)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

// при превышении лимита убирается ровно excess сессий: если хватает дублей, остальные дубли НЕ трогаются.
func TestSessionExcessTrimmer_Execute_RevokesOnlyExcessNotAllDuplicates(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	m := newExcessTrimmerMocks(ctrl)

	userID := uuid.New()
	now := time.Now()

	// лимит 4; 6 сессий с двух устройств (A×5, B×1). Нужно убрать excess = 6-4 = 2,
	// поэтому ревокаются только 2 наименее активных дубля A (s4, s5), а s2/s3 остаются.
	sessions := []entity.Session{
		{UserID: userID, SessionID: 1, UserAgent: "A", CreatedAt: now.Add(-1 * time.Minute)},
		{UserID: userID, SessionID: 2, UserAgent: "A", CreatedAt: now.Add(-2 * time.Minute)},
		{UserID: userID, SessionID: 3, UserAgent: "A", CreatedAt: now.Add(-3 * time.Minute)},
		{UserID: userID, SessionID: 4, UserAgent: "A", CreatedAt: now.Add(-4 * time.Minute)},
		{UserID: userID, SessionID: 5, UserAgent: "A", CreatedAt: now.Add(-5 * time.Minute)},
		{UserID: userID, SessionID: 6, UserAgent: "B", CreatedAt: now.Add(-6 * time.Minute)},
	}

	var gotRevoke []uint32

	gomock.InOrder(
		m.consumer.EXPECT().Fetch(gomock.Any(), 100).
			Return([]entity.SessionExcessItem{{UserID: userID, RealmID: testRealmID, SessionMax: 4}}, nil),
		m.openFetcher.EXPECT().FetchOpenSessionIDs(gomock.Any(), userID, testRealmID).Return([]uint32{1, 2, 3, 4, 5, 6}, nil),
		m.lister.EXPECT().FetchOrderedListByUserIDAndSessionIDs(gomock.Any(), userID, []uint32{1, 2, 3, 4, 5, 6}, 0).Return(sessions, nil),
		m.closer.EXPECT().RevokeTokensBySessionIDs(gomock.Any(), userID, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uuid.UUID, ids []uint32) error {
				gotRevoke = ids

				return nil
			}),
		m.deleter.EXPECT().DeleteOrphaned(gomock.Any(), gomock.Any()).Return(nil),
		m.consumer.EXPECT().Delete(gomock.Any(), []entity.SessionExcessPK{{UserID: userID, RealmID: testRealmID}}).Return(nil),
	)

	count, err := m.uc.Execute(context.Background(), 100)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	require.ElementsMatch(t, []uint32{4, 5}, gotRevoke)
}

// пользователь без открытых сессий пропускается (без обращения к списку), но снимается с очереди.
func TestSessionExcessTrimmer_Execute_NoOpenSessionsSkipped(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	m := newExcessTrimmerMocks(ctrl)

	userID := uuid.New()

	gomock.InOrder(
		m.consumer.EXPECT().Fetch(gomock.Any(), 100).
			Return([]entity.SessionExcessItem{{UserID: userID, RealmID: testRealmID, SessionMax: 4}}, nil),
		m.openFetcher.EXPECT().FetchOpenSessionIDs(gomock.Any(), userID, testRealmID).Return([]uint32{}, nil),
		m.consumer.EXPECT().Delete(gomock.Any(), []entity.SessionExcessPK{{UserID: userID, RealmID: testRealmID}}).Return(nil),
	)
	// FetchOrderedListByUserIDAndSessionIDs не вызывается

	count, err := m.uc.Execute(context.Background(), 100)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

// сбой ревока прерывает обработку до ack (at-least-once: пользователь останется в очереди).
func TestSessionExcessTrimmer_Execute_RevokeErrorSkipsAck(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	m := newExcessTrimmerMocks(ctrl)

	userID := uuid.New()
	now := time.Now()

	sessions := []entity.Session{
		{UserID: userID, SessionID: 1, UserAgent: "A", CreatedAt: now.Add(-1 * time.Minute)},
		{UserID: userID, SessionID: 2, UserAgent: "B", CreatedAt: now.Add(-2 * time.Minute)},
	}

	m.consumer.EXPECT().Fetch(gomock.Any(), 100).
		Return([]entity.SessionExcessItem{{UserID: userID, RealmID: testRealmID, SessionMax: 1}}, nil)
	m.openFetcher.EXPECT().FetchOpenSessionIDs(gomock.Any(), userID, testRealmID).Return([]uint32{1, 2}, nil)
	m.lister.EXPECT().FetchOrderedListByUserIDAndSessionIDs(gomock.Any(), userID, []uint32{1, 2}, 0).Return(sessions, nil)
	m.closer.EXPECT().RevokeTokensBySessionIDs(gomock.Any(), userID, gomock.Any()).
		Return(errors.New("revoke failed"))
	// DeleteOrphaned и consumer.Delete (ack) не должны вызываться

	_, err := m.uc.Execute(context.Background(), 100)
	require.Error(t, err)
}
