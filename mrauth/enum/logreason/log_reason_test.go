package logreason_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/enum/logreason"
)

func TestString(t *testing.T) {
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
		{"ResendsExhausted", logreason.ResendsExhausted, "RESENDS_EXHAUSTED"},
		{"Auth2FAStateChanged", logreason.Auth2FAStateChanged, "AUTH_2FA_STATE_CHANGED"},
		{"EmailChanged", logreason.EmailChanged, "EMAIL_CHANGED"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, c.str, c.value.String())
		})
	}
}

func TestStringUnknown(t *testing.T) {
	t.Parallel()

	require.Equal(t, "UNKNOWN", logreason.Enum(uint8(logreason.EmailChanged)+1).String())
}
