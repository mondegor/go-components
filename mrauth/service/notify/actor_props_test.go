package notify_test

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/mondegor/go-core/mrtype"
	"github.com/mondegor/go-core/util/conv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/dto"
	"github.com/mondegor/go-components/mrauth/service/notify"
)

func TestActorProps_With(t *testing.T) {
	t.Parallel()

	// адрес из заголовков прокси в уведомление не попадает - только реальный IP
	actor := dto.ActorMeta{
		ClientIP:  mrtype.NewDetailedIP(netip.MustParseAddr("192.0.2.10"), netip.MustParseAddr("198.51.100.7")),
		UserAgent: "test-agent",
	}

	type testCase struct {
		name       string
		appName    string
		deviceName string
		nilApp     bool
		wantDevice string
	}

	tests := []testCase{
		{name: "app and device", appName: "Chrome", deviceName: "Windows", wantDevice: "Chrome, Windows"},
		{name: "only device", deviceName: "iPhone", wantDevice: "iPhone"},
		{name: "only app", appName: "Firefox", wantDevice: "Firefox"},
		{name: "nothing resolved", wantDevice: ""},
		{name: "resolver not set", nilApp: true, wantDevice: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotUserAgent string

			appResolver := func(userAgent string) (string, string) {
				gotUserAgent = userAgent

				return tt.appName, tt.deviceName
			}

			if tt.nilApp {
				appResolver = nil
			}

			props := conv.Group{"to": "user@example.com"}
			got := notify.NewActorProps(appResolver).With(actor, props)

			assert.Equal(t, conv.Group{"to": "user@example.com"}, props, "source props must not change")
			assert.Equal(t, "user@example.com", got["to"])
			assert.Equal(t, "192.0.2.10", got["ip"])
			assert.Equal(t, tt.wantDevice, got["device"])

			if !tt.nilApp {
				assert.Equal(t, "test-agent", gotUserAgent)
			}

			occurredAt, ok := got["occurredAt"].(string)
			require.True(t, ok)
			assert.True(t, strings.HasSuffix(occurredAt, " (UTC)"), occurredAt)
		})
	}
}

func TestActorProps_FormatTime(t *testing.T) {
	t.Parallel()

	moscow, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)

	tm := time.Date(2026, 9, 30, 21, 5, 0, 0, time.UTC)

	tests := []struct {
		name  string
		actor dto.ActorMeta
		want  string
	}{
		{
			name:  "user location",
			actor: dto.NewAnonymousActorMeta(mrtype.DetailedIP{}, "", moscow),
			want:  "2026-10-01 00:05 (Europe/Moscow)",
		},
		{
			name:  "location not set is UTC",
			actor: dto.ActorMeta{},
			want:  "2026-09-30 21:05 (UTC)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, notify.NewActorProps(nil).FormatTime(tt.actor, tm))
		})
	}
}
