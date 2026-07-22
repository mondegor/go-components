package logreason_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/enum/logreason"
)

func TestParseStringRoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value logreason.Enum
		str   string
	}{
		{"Unspecified", logreason.Unspecified, "UNSPECIFIED"},
		{"WrongCode", logreason.WrongCode, "WRONG_CODE"},
		{"AttemptsExhausted", logreason.AttemptsExhausted, "ATTEMPTS_EXHAUSTED"},
		{"Throttled", logreason.Throttled, "THROTTLED"},
		{"TokenReuse", logreason.TokenReuse, "TOKEN_REUSE"},
		{"AccessForbidden", logreason.AccessForbidden, "ACCESS_FORBIDDEN"},
		{"TOTPReplay", logreason.TOTPReplay, "TOTP_REPLAY"},
		{"Expired", logreason.Expired, "EXPIRED"},
		{"NotConfirmed", logreason.NotConfirmed, "NOT_CONFIRMED"},
		{"LoginNotExists", logreason.LoginNotExists, "LOGIN_NOT_EXISTS"},
		{"SessionLimit", logreason.SessionLimit, "SESSION_LIMIT"},
		{"Superseded", logreason.Superseded, "SUPERSEDED"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, c.str, c.value.String())

			parsed, err := logreason.Parse(c.str)
			require.NoError(t, err)
			require.Equal(t, c.value, parsed)

			// Scan(int64) должен восстановить то же значение (в т.ч. 0 = UNSPECIFIED)
			var scanned logreason.Enum
			require.NoError(t, scanned.Scan(int64(c.value)))
			require.Equal(t, c.value, scanned)

			// JSON round-trip
			data, err := json.Marshal(c.value)
			require.NoError(t, err)
			require.JSONEq(t, `"`+c.str+`"`, string(data))

			var unmarshaled logreason.Enum
			require.NoError(t, json.Unmarshal(data, &unmarshaled))
			require.Equal(t, c.value, unmarshaled)
		})
	}
}

func TestSetBounds(t *testing.T) {
	t.Parallel()

	var e logreason.Enum

	// 0 (UNSPECIFIED) - валидное значение
	require.NoError(t, e.Set(0))
	require.Equal(t, logreason.Unspecified, e)

	// за верхней границей
	require.Error(t, e.Set(uint8(logreason.Superseded)+1))
}

func TestParseInvalid(t *testing.T) {
	t.Parallel()

	_, err := logreason.Parse("NOPE")
	require.Error(t, err)
}
