package logstatus_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/enum/logstatus"
)

func TestParseStringRoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value logstatus.Enum
		str   string
	}{
		{"Opened", logstatus.Opened, "OPENED"},
		{"ResentCode", logstatus.ResentCode, "RESENT_CODE"},
		{"ConfirmSuccess", logstatus.ConfirmSuccess, "CONFIRM_SUCCESS"},
		{"ConfirmFailed", logstatus.ConfirmFailed, "CONFIRM_FAILED"},
		{"Confirmed", logstatus.Confirmed, "CONFIRMED"},
		{"Revoked", logstatus.Revoked, "REVOKED"},
		{"Applied", logstatus.Applied, "APPLIED"},
		{"Blocked", logstatus.Blocked, "BLOCKED"},
		{"SessionOpened", logstatus.SessionOpened, "SESSION_OPENED"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, c.str, c.value.String())

			parsed, err := logstatus.Parse(c.str)
			require.NoError(t, err)
			require.Equal(t, c.value, parsed)

			// Scan(int64) должен восстановить то же значение
			var scanned logstatus.Enum
			require.NoError(t, scanned.Scan(int64(c.value)))
			require.Equal(t, c.value, scanned)

			// JSON round-trip
			data, err := json.Marshal(c.value)
			require.NoError(t, err)
			require.JSONEq(t, `"`+c.str+`"`, string(data))

			var unmarshaled logstatus.Enum
			require.NoError(t, json.Unmarshal(data, &unmarshaled))
			require.Equal(t, c.value, unmarshaled)
		})
	}
}

func TestSetBounds(t *testing.T) {
	t.Parallel()

	var e logstatus.Enum

	// 0 не входит в набор (нумерация с 1)
	require.Error(t, e.Set(0))
	// за верхней границей
	require.Error(t, e.Set(uint8(logstatus.SessionOpened)+1))

	require.NoError(t, e.Set(uint8(logstatus.Opened)))
	require.Equal(t, logstatus.Opened, e)
}

func TestStringUnknown(t *testing.T) {
	t.Parallel()

	require.Equal(t, "UNKNOWN", logstatus.Enum(0).String())
}

func TestParseInvalid(t *testing.T) {
	t.Parallel()

	_, err := logstatus.Parse("NOPE")
	require.Error(t, err)
}
