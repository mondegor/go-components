package security_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	sysmesserrors "github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/mrtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/securityevent"
	"github.com/mondegor/go-components/mrauth/usecase/security"
	"github.com/mondegor/go-components/mrauth/usecase/security/mock"
)

//go:generate mockgen -source=security_log.go -destination=mock/security_log.go -package=mock

// TestGetSecurityLog_Execute - записи журнала отдаются в порядке хранилища с приложением
// и устройством по user agent, IP и местоположением по адресу записи; подробности переносятся
// только у событий, где они есть; признак продолжения передаётся как есть.
func TestGetSecurityLog_Execute(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	storage := mock.NewMocksecurityLogFetcher(ctrl)

	userID := uuid.New()
	cursor := mrstorage.IDCursor{ID: 10, Limit: 2}
	at := time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC)
	remaining := 3

	storage.EXPECT().FetchByUserID(gomock.Any(), userID, cursor).Return(
		[]entity.SecurityLogEvent{
			{
				RecordID:  9,
				UserID:    userID,
				EventType: securityevent.EmailChanged,
				ClientIP:  mrtype.NewIP(netip.MustParseAddr("192.0.2.1")),
				UserAgent: "agent-1",
				Extra:     &entity.SecurityLogExtra{OldValue: "old@example.com", NewValue: "new@example.com"},
				CreatedAt: at,
			},
			{
				RecordID:  8,
				UserID:    userID,
				EventType: securityevent.RecoveryCodeUsed,
				ClientIP:  mrtype.NewIP(netip.MustParseAddr("192.0.2.2")),
				UserAgent: "agent-2",
				Extra:     &entity.SecurityLogExtra{Remaining: &remaining},
				CreatedAt: at,
			},
			{
				RecordID:  7,
				UserID:    userID,
				EventType: securityevent.SignedIn,
				ClientIP:  mrtype.NewIP(netip.MustParseAddr("192.0.2.3")),
				CreatedAt: at,
			},
		},
		true,
		nil,
	)

	uc := security.NewGetSecurityLog(
		storage,
		func(userAgent string) (string, string) {
			return "app:" + userAgent, "device:" + userAgent
		},
		func(ip netip.Addr, mode mrauth.LocationMode) string {
			if mode == mrauth.LocationOnlyIP {
				return ip.String()
			}

			return "loc:" + ip.String()
		},
	)

	items, hasNext, err := uc.Execute(context.Background(), userID, cursor)
	require.NoError(t, err)
	assert.True(t, hasNext)
	assert.Equal(
		t,
		[]dto.SecurityLogItem{
			{
				RecordID:   9,
				EventType:  securityevent.EmailChanged,
				AppName:    "app:agent-1",
				DeviceName: "device:agent-1",
				IP:         "192.0.2.1",
				Location:   "loc:192.0.2.1",
				OldValue:   "old@example.com",
				NewValue:   "new@example.com",
				CreatedAt:  at,
			},
			{
				RecordID:   8,
				EventType:  securityevent.RecoveryCodeUsed,
				AppName:    "app:agent-2",
				DeviceName: "device:agent-2",
				IP:         "192.0.2.2",
				Location:   "loc:192.0.2.2",
				Remaining:  &remaining,
				CreatedAt:  at,
			},
			{
				RecordID:   7,
				EventType:  securityevent.SignedIn,
				AppName:    "app:",
				DeviceName: "device:",
				IP:         "192.0.2.3",
				Location:   "loc:192.0.2.3",
				CreatedAt:  at,
			},
		},
		items,
	)
}

// TestGetSecurityLog_ExecuteDefaultResolvers - без резолверов хоста приложение и устройство
// пустые, а вместо местоположения - только IP по умолчанию.
func TestGetSecurityLog_ExecuteDefaultResolvers(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	storage := mock.NewMocksecurityLogFetcher(ctrl)

	storage.EXPECT().FetchByUserID(gomock.Any(), gomock.Any(), gomock.Any()).Return(
		[]entity.SecurityLogEvent{
			{RecordID: 1, EventType: securityevent.SignedIn, ClientIP: mrtype.NewIP(netip.MustParseAddr("192.0.2.1")), UserAgent: "agent"},
		},
		false,
		nil,
	)

	items, hasNext, err := security.NewGetSecurityLog(storage, nil, nil).Execute(context.Background(), uuid.New(), mrstorage.IDCursor{Limit: 1})
	require.NoError(t, err)
	assert.False(t, hasNext)
	require.Len(t, items, 1)
	assert.Empty(t, items[0].AppName)
	assert.Empty(t, items[0].DeviceName)
	assert.Equal(t, "192.0.2.1", items[0].IP)
	assert.Empty(t, items[0].Location)
}

// TestGetSecurityLog_ExecuteErrors - метод доступен только залогиненным, поэтому пустой userID -
// ошибка проводки (хранилище не вызывается); сбой хранилища возвращается наружу.
func TestGetSecurityLog_ExecuteErrors(t *testing.T) {
	t.Parallel()

	t.Run("empty user", func(t *testing.T) {
		t.Parallel()

		uc := security.NewGetSecurityLog(mock.NewMocksecurityLogFetcher(gomock.NewController(t)), nil, nil)

		_, _, err := uc.Execute(context.Background(), uuid.Nil, mrstorage.IDCursor{Limit: 1})
		require.ErrorIs(t, err, sysmesserrors.ErrInternalIncorrectInputData)
	})

	t.Run("storage error", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("fetch failed")

		storage := mock.NewMocksecurityLogFetcher(gomock.NewController(t))
		storage.EXPECT().FetchByUserID(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false, wantErr)

		items, hasNext, err := security.NewGetSecurityLog(storage, nil, nil).Execute(context.Background(), uuid.New(), mrstorage.IDCursor{Limit: 1})
		require.ErrorIs(t, err, wantErr)
		assert.Nil(t, items)
		assert.False(t, hasNext)
	})
}
