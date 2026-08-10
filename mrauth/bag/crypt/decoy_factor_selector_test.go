package crypt_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/bag/crypt"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
)

// testDecoySalt - соль ровно минимальной длины (32 байта).
const testDecoySalt = "test-decoy-salt-0123456789abcdef"

func TestNewDecoyFactorSelector(t *testing.T) {
	t.Parallel()

	// причина отказа проверяется по подстроке сообщения: ошибки конструктора - это
	// валидация настроек инсталляции, сентинелов для них в пакете нет. Числа границ
	// в подстроки не входят намеренно - иначе тест ломался бы при их правке
	type testCase struct {
		name            string
		salt            []byte
		totpPercent     uint32
		wantErrContains string
	}

	tests := []testCase{
		{name: "пустая соль", salt: nil, totpPercent: 50, wantErrContains: "salt is too short"},
		{name: "короткая соль", salt: []byte(testDecoySalt[:len(testDecoySalt)-1]), totpPercent: 50, wantErrContains: "salt is too short"},
		{name: "нулевая доля", salt: []byte(testDecoySalt), totpPercent: 0, wantErrContains: "totpPercent is out of range"},
		{name: "полная доля", salt: []byte(testDecoySalt), totpPercent: 100, wantErrContains: "totpPercent is out of range"},
		{name: "доля вне диапазона", salt: []byte(testDecoySalt), totpPercent: 101, wantErrContains: "totpPercent is out of range"},
		{name: "нижняя граница", salt: []byte(testDecoySalt), totpPercent: 1},
		{name: "доля по умолчанию", salt: []byte(testDecoySalt), totpPercent: crypt.DefaultTOTPPercent()},
		{name: "верхняя граница", salt: []byte(testDecoySalt), totpPercent: 99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			selector, err := crypt.NewDecoyFactorSelector(tt.salt, tt.totpPercent)

			if tt.wantErrContains != "" {
				require.ErrorContains(t, err, tt.wantErrContains)
				require.Nil(t, selector)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, selector)
		})
	}
}

func TestDecoyFactorSelector_Select(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("bf1b21fb-4b8a-4c05-96f5-3ff6c5a8c1de")

	selector, err := crypt.NewDecoyFactorSelector([]byte(testDecoySalt), crypt.DefaultTOTPPercent())
	require.NoError(t, err)

	got := selector.Select(userID)
	require.Contains(t, []auth2fatype.Enum{auth2fatype.Password, auth2fatype.TOTP}, got)

	// тип обязан быть постоянным: иначе повторный вызов метода выдал бы подставную цепочку
	require.Equal(t, got, selector.Select(userID))

	// при известной соли тип предсказуем, поэтому соль обязана быть секретом инсталляции
	other, err := crypt.NewDecoyFactorSelector([]byte(testDecoySalt), crypt.DefaultTOTPPercent())
	require.NoError(t, err)
	require.Equal(t, got, other.Select(userID))
}

func TestDecoyFactorSelector_SelectDistribution(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name        string
		totpPercent uint32
		want        auth2fatype.Enum
	}

	// края диапазона недопустимы, но соседние с ними доли дают практически вырожденное
	// распределение - на нём и проверяется, что доля вообще участвует в выборе
	tests := []testCase{
		{name: "почти всегда пароль", totpPercent: 1, want: auth2fatype.Password},
		{name: "почти всегда TOTP", totpPercent: 99, want: auth2fatype.TOTP},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			selector, err := crypt.NewDecoyFactorSelector([]byte(testDecoySalt), tt.totpPercent)
			require.NoError(t, err)

			const total = 200

			matched := 0

			for range total {
				if selector.Select(uuid.New()) == tt.want {
					matched++
				}
			}

			require.Greater(t, matched, total/2)
		})
	}
}
