package crypt_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/bag/crypt"
)

func TestSecretGenerator_GenToken(t *testing.T) {
	t.Parallel()

	gen := crypt.NewSecretGenerator()

	token, err := gen.GenToken(32)
	require.NoError(t, err)
	require.Len(t, token, 32)

	other, err := gen.GenToken(32)
	require.NoError(t, err)
	require.NotEqual(t, token, other) // токены должны быть случайными
}

func TestSecretGenerator_GenCode(t *testing.T) {
	t.Parallel()

	code, err := crypt.NewSecretGenerator().GenCode(6)
	require.NoError(t, err)
	require.Len(t, code, 6)

	for _, r := range code {
		require.True(t, r >= '0' && r <= '9', "ожидались только цифры, получено %q", code)
	}
}

func TestSecretGenerator_GenRecoveryCode(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		length        int
		wantSeparator bool
	}

	tests := []testCase{
		{name: "короткий без разделителя", length: 8, wantSeparator: false},
		{name: "длинный с разделителем", length: 17, wantSeparator: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			code, err := crypt.NewSecretGenerator().GenRecoveryCode(tt.length)
			require.NoError(t, err)
			require.Len(t, code, tt.length)

			if tt.wantSeparator {
				require.Equal(t, byte('-'), code[tt.length/2])
				require.Equal(t, 1, strings.Count(code, "-"))

				return
			}

			require.NotContains(t, code, "-")
		})
	}
}

func TestSecretGenerator_HashAndCompare(t *testing.T) {
	t.Parallel()

	gen := crypt.NewSecretGenerator()

	hash, err := gen.HashedSecret("my-secret")
	require.NoError(t, err)
	require.NotEqual(t, "my-secret", hash)

	ok, err := gen.CompareSecretAndHash("my-secret", hash)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = gen.CompareSecretAndHash("wrong-secret", hash)
	require.NoError(t, err) // несовпадение секрета - это не ошибка
	require.False(t, ok)
}

func TestSecretGenerator_GenerateRecoveryCodes(t *testing.T) {
	t.Parallel()

	gen := crypt.NewSecretGenerator()

	plain, hashed, err := gen.GenerateRecoveryCodes(5, 12)
	require.NoError(t, err)
	require.Len(t, plain, 5)
	require.Len(t, hashed, 5)

	for i := range plain {
		require.Len(t, plain[i], 12)

		require.NotEqual(t, plain[i], hashed[i]) // хранится хеш, не открытый код

		ok, err := gen.CompareSecretAndHash(plain[i], hashed[i])
		require.NoError(t, err)
		require.True(t, ok)
	}
}

func TestIsRecoveryCodeFormat(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name string
		code string
		want bool
	}

	tests := []testCase{
		{name: "с разделителем посередине", code: "ABCD1234-EFGH5678", want: true},
		{name: "короче длины с разделителем", code: "ABCD1234", want: false},
		{name: "пустая строка", code: "", want: false},
		{name: "нет разделителя у длинного", code: "ABCD1234EEFGH5678", want: false},
		{name: "разделитель не посередине", code: "ABC-D1234EFGH5678", want: false},
		{name: "два разделителя", code: "ABCD-234-EFGH5678", want: false},
		{name: "строчные буквы", code: "abcd1234-efgh5678", want: false},
		{name: "одна строчная буква", code: "ABCD1234-EFGh5678", want: false},
		{name: "посторонний символ", code: "ABCD1234-EFGH567!", want: false},
		{name: "разделитель у короткого", code: "ABCD-234", want: false},
		{name: "TOTP-код", code: "123456", want: false},
		{name: "цифровой код подтверждения", code: "18394712", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, crypt.IsRecoveryCodeFormat(tt.code))
		})
	}
}

func TestIsRecoveryCodeFormat_GeneratedCodes(t *testing.T) {
	t.Parallel()

	for _, length := range []int{11, 16, 17, 32} {
		code, err := crypt.NewSecretGenerator().GenRecoveryCode(length)
		require.NoError(t, err)
		assert.True(t, crypt.IsRecoveryCodeFormat(code), code)
	}
}
