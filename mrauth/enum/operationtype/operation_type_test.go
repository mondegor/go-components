package operationtype_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/enum/operationtype"
)

// TestParseStringRoundTrip - строки клиентских типов совпадают с enum контракта Auth.Enum.OperationType,
// строки остальных типов переживают разбор и сериализацию.
func TestParseStringRoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value operationtype.Enum
		str   string
	}{
		{"AuthorizeUser", operationtype.AuthorizeUser, "AUTHORIZE_USER"},
		{"CreateUser", operationtype.CreateUser, "CREATE_USER"},
		{"ChangeEmail", operationtype.ChangeEmail, "CHANGE_EMAIL"},
		{"ChangeEmailConfirm", operationtype.ChangeEmailConfirm, "CHANGE_EMAIL_CONFIRM"},
		{"ChangePhone", operationtype.ChangePhone, "CHANGE_PHONE"},
		{"ChangePassword", operationtype.ChangePassword, "CHANGE_PASSWORD"},
		{"ChangeTOTP", operationtype.ChangeTOTP, "CHANGE_TOTP"},
		{"Disable2FA", operationtype.Disable2FA, "DISABLE_2FA"},
		{"RegenerateRecovery", operationtype.RegenerateRecovery, "REGENERATE_RECOVERY"},
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

			var unmarshaled operationtype.Enum

			require.NoError(t, json.Unmarshal(data, &unmarshaled))
			require.Equal(t, c.value, unmarshaled)
		})
	}
}

func TestSetBounds(t *testing.T) {
	t.Parallel()

	var e operationtype.Enum

	// 0 не входит в набор (нумерация с 1)
	require.Error(t, e.Set(0))
	// за верхней границей
	require.Error(t, e.Set(uint8(operationtype.RegenerateRecovery)+1))

	require.NoError(t, e.Set(uint8(operationtype.AuthorizeUser)))
	require.Equal(t, operationtype.AuthorizeUser, e)
}

// TestUnknown - неизвестное значение не парсится, нулевое значение печатается как UNKNOWN.
func TestUnknown(t *testing.T) {
	t.Parallel()

	_, err := operationtype.Parse("NOPE")
	require.Error(t, err)
	require.Equal(t, "UNKNOWN", operationtype.Enum(0).String())
}
