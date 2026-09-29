package crypt_test

import (
	"strings"
	"testing"

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
