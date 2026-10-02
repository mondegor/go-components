package passwordacceptstatus_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/enum/passwordacceptstatus"
)

func TestStringAndMarshalJSON(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value passwordacceptstatus.Enum
		str   string
	}{
		{"Accepted", passwordacceptstatus.Accepted, "ACCEPTED"},
		{"TooWeak", passwordacceptstatus.TooWeak, "TOO_WEAK"},
		{"RecoveryCodeFormat", passwordacceptstatus.RecoveryCodeFormat, "RECOVERY_CODE_FORMAT"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, c.str, c.value.String())

			data, err := json.Marshal(c.value)
			require.NoError(t, err)
			require.JSONEq(t, `"`+c.str+`"`, string(data))
		})
	}
}

func TestStringUnknown(t *testing.T) {
	t.Parallel()

	require.Equal(t, "UNKNOWN", passwordacceptstatus.Enum(0).String())
}
