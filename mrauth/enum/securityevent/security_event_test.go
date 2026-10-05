package securityevent_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/enum/securityevent"
)

// TestStringMarshalScan - строки типов совпадают с enum контракта, все типы сериализуются
// в JSON строкой и переживают запись в хранилище и чтение из него.
func TestStringMarshalScan(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value securityevent.Enum
		str   string
	}{
		{"SignedIn", securityevent.SignedIn, "SIGNED_IN"},
		{"Auth2FAEnabled", securityevent.Auth2FAEnabled, "AUTH_2FA_ENABLED"},
		{"Auth2FADisabled", securityevent.Auth2FADisabled, "AUTH_2FA_DISABLED"},
		{"RecoveryCodeUsed", securityevent.RecoveryCodeUsed, "RECOVERY_CODE_USED"},
		{"RecoveryCodesRegenerated", securityevent.RecoveryCodesRegenerated, "RECOVERY_CODES_REGENERATED"},
		{"EmailChangeRequested", securityevent.EmailChangeRequested, "EMAIL_CHANGE_REQUESTED"},
		{"EmailChanged", securityevent.EmailChanged, "EMAIL_CHANGED"},
		{"EmailChangeRevoked", securityevent.EmailChangeRevoked, "EMAIL_CHANGE_REVOKED"},
		{"PhoneChanged", securityevent.PhoneChanged, "PHONE_CHANGED"},
		{"SessionsClosed", securityevent.SessionsClosed, "SESSIONS_CLOSED"},
		{"TokenReuseDetected", securityevent.TokenReuseDetected, "TOKEN_REUSE_DETECTED"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, c.str, c.value.String())

			data, err := json.Marshal(c.value)
			require.NoError(t, err)
			require.JSONEq(t, `"`+c.str+`"`, string(data))

			dbValue, err := c.value.Value()
			require.NoError(t, err)
			require.IsType(t, uint8(0), dbValue)

			var scanned securityevent.Enum

			require.NoError(t, scanned.Scan(int64(dbValue.(uint8))))
			require.Equal(t, c.value, scanned)
		})
	}
}

func TestSetBounds(t *testing.T) {
	t.Parallel()

	var e securityevent.Enum

	// 0 не входит в набор (нумерация с 1)
	require.Error(t, e.Set(0))
	// за верхней границей
	require.Error(t, e.Set(uint8(securityevent.TokenReuseDetected)+1))

	require.NoError(t, e.Set(uint8(securityevent.SignedIn)))
	require.Equal(t, securityevent.SignedIn, e)
}

// TestUnknown - нулевое значение печатается как UNKNOWN.
func TestUnknown(t *testing.T) {
	t.Parallel()

	require.Equal(t, "UNKNOWN", securityevent.Enum(0).String())
}
