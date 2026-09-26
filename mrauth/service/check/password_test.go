package check_test

import (
	"testing"

	"github.com/mondegor/go-core/util/crypt/password"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/service/check"
)

// TestPassword_Generate - сгенерированный пароль всегда имеет заданную длину и надёжность THE_BEST.
func TestPassword_Generate(t *testing.T) {
	t.Parallel()

	const (
		length     = 16
		iterations = 1000
	)

	sv := check.NewPassword(length)

	for range iterations {
		userPassword, err := sv.Generate()
		require.NoError(t, err)

		assert.Len(t, userPassword, length)
		strength, acceptable := sv.CalcStrength(userPassword)
		assert.Equal(t, "THE_BEST", strength, userPassword)
		assert.True(t, acceptable, userPassword)
	}
}

// TestPassword_GenerateUnreachableStrength - при длине, на которой THE_BEST недостижим,
// генерация завершается ошибкой, а не отдаёт более слабый пароль.
func TestPassword_GenerateUnreachableStrength(t *testing.T) {
	t.Parallel()

	userPassword, err := check.NewPassword(8).Generate()
	require.Error(t, err)
	assert.Empty(t, userPassword)
}

// TestPassword_MinStrength - порог надёжности пароля 2FA задаётся опцией, а порог
// по умолчанию отклоняет пароль, прошедший лишь проверку длины и набора символов на границе
// ввода. Оценка для клиента и проверка при установке пароля дают одинаковый ответ.
func TestPassword_MinStrength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		opts         []check.PasswordOption
		userPassword string
		want         bool
	}{
		{name: "default threshold: one char class", userPassword: "aaaaaaaa", want: false},
		{name: "default threshold: medium", userPassword: "abcdefgh1234", want: false},
		{name: "default threshold: strong", userPassword: "abcdEFGH1234", want: true},
		{
			name:         "threshold THE_BEST: strong is not enough",
			opts:         []check.PasswordOption{check.WithMinStrength(password.PassStrengthBest)},
			userPassword: "abcdEFGH1234",
			want:         false,
		},
		{
			name:         "threshold MIDDLE: medium is enough",
			opts:         []check.PasswordOption{check.WithMinStrength(password.PassStrengthMedium)},
			userPassword: "abcdefgh1234",
			want:         true,
		},
		{
			name:         "NOT_RATED threshold falls back to default",
			opts:         []check.PasswordOption{check.WithMinStrength(password.PassStrengthNotRated)},
			userPassword: "abcdefgh1234",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sv := check.NewPassword(16, tt.opts...)

			_, acceptable := sv.CalcStrength(tt.userPassword)
			assert.Equal(t, tt.want, acceptable)
			assert.Equal(t, tt.want, sv.IsAcceptable(tt.userPassword))
		})
	}
}
