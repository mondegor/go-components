package logstatus_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/enum/logstatus"
)

func TestString(t *testing.T) {
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
		})
	}
}

func TestStringUnknown(t *testing.T) {
	t.Parallel()

	require.Equal(t, "UNKNOWN", logstatus.Enum(uint8(logstatus.SessionOpened)+1).String())
}
