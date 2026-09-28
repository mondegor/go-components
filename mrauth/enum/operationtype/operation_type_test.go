package operationtype_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/enum/operationtype"
)

// TestParseStringRoundTrip - строки типов совпадают с enum контракта Auth.Enum.OperationType.
func TestParseStringRoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value operationtype.Enum
		str   string
	}{
		{"ChangeEmail", operationtype.ChangeEmail, "CHANGE_EMAIL"},
		{"ChangeEmailConfirm", operationtype.ChangeEmailConfirm, "CHANGE_EMAIL_CONFIRM"},
		{"Disable2FA", operationtype.Disable2FA, "DISABLE_2FA"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, c.str, c.value.String())

			parsed, err := operationtype.Parse(c.str)
			require.NoError(t, err)
			require.Equal(t, c.value, parsed)

			data, err := json.Marshal(c.value)
			require.NoError(t, err)
			require.JSONEq(t, `"`+c.str+`"`, string(data))
		})
	}
}

// TestUnknown - неизвестное значение не парсится, нулевое значение печатается как UNKNOWN.
func TestUnknown(t *testing.T) {
	t.Parallel()

	_, err := operationtype.Parse("CREATE_USER")
	require.Error(t, err)
	require.Equal(t, "UNKNOWN", operationtype.Enum(0).String())
}
