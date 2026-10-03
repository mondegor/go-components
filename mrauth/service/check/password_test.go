package check_test

import (
	"testing"

	"github.com/mondegor/go-core/util/crypt/password"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/enum/passwordacceptstatus"
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
		strength, acceptStatus := sv.CalcStrength(userPassword)
		assert.Equal(t, "THE_BEST", strength, userPassword)
		assert.Equal(t, passwordacceptstatus.Accepted, acceptStatus, userPassword)
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
// ввода. Формат аварийного кода отклоняется своей причиной даже ниже порога. Оценка для клиента
// и проверка при установке пароля дают одинаковый ответ, а уровень надёжности от причины не зависит.
func TestPassword_MinStrength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		opts         []check.PasswordOption
		userPassword string
		wantStatus   passwordacceptstatus.Enum
		wantErr      error
	}{
		{
			name:         "default threshold: one char class",
			userPassword: "aaaaaaaaaa",
			wantStatus:   passwordacceptstatus.TooWeak,
			wantErr:      mrauth.ErrPasswordIsTooWeak,
		},
		{
			name:         "default threshold: medium",
			userPassword: "abcdefgh1234",
			wantStatus:   passwordacceptstatus.TooWeak,
			wantErr:      mrauth.ErrPasswordIsTooWeak,
		},
		{
			name:         "default threshold: 10 chars of all classes, 9 unique",
			userPassword: "abcDEF12!!",
			wantStatus:   passwordacceptstatus.TooWeak,
			wantErr:      mrauth.ErrPasswordIsTooWeak,
		},
		{name: "default threshold: 10 unique chars of all classes", userPassword: "abcDEF12!?", wantStatus: passwordacceptstatus.Accepted},
		{name: "default threshold: strong", userPassword: "abcdEFGH1234", wantStatus: passwordacceptstatus.Accepted},
		{
			name:         "threshold THE_BEST: strong is not enough",
			opts:         []check.PasswordOption{check.WithMinStrength(password.PassStrengthBest)},
			userPassword: "abcdEFGH1234",
			wantStatus:   passwordacceptstatus.TooWeak,
			wantErr:      mrauth.ErrPasswordIsTooWeak,
		},
		{
			name:         "threshold MIDDLE: medium is enough",
			opts:         []check.PasswordOption{check.WithMinStrength(password.PassStrengthMedium)},
			userPassword: "abcdefgh1234",
			wantStatus:   passwordacceptstatus.Accepted,
		},
		{
			name:         "recovery code format is never acceptable",
			opts:         []check.PasswordOption{check.WithMinStrength(password.PassStrengthMedium)},
			userPassword: "ABCD1234-EFGH5678",
			wantStatus:   passwordacceptstatus.RecoveryCodeFormat,
			wantErr:      mrauth.ErrPasswordHasRecoveryCodeFormat,
		},
		{
			name:         "recovery code format below threshold is reported as format",
			opts:         []check.PasswordOption{check.WithMinStrength(password.PassStrengthBest)},
			userPassword: "ABCD1234-EFGH5678",
			wantStatus:   passwordacceptstatus.RecoveryCodeFormat,
			wantErr:      mrauth.ErrPasswordHasRecoveryCodeFormat,
		},
		{
			name:         "recovery code format with one lowercase letter is acceptable",
			opts:         []check.PasswordOption{check.WithMinStrength(password.PassStrengthMedium)},
			userPassword: "ABCD1234-EFGh5678",
			wantStatus:   passwordacceptstatus.Accepted,
		},
		{
			name:         "dash not in the middle is acceptable",
			opts:         []check.PasswordOption{check.WithMinStrength(password.PassStrengthMedium)},
			userPassword: "ABCD-1234EFGH5678",
			wantStatus:   passwordacceptstatus.Accepted,
		},
		{
			name:         "NOT_RATED threshold falls back to default",
			opts:         []check.PasswordOption{check.WithMinStrength(password.PassStrengthNotRated)},
			userPassword: "abcdefgh1234",
			wantStatus:   passwordacceptstatus.TooWeak,
			wantErr:      mrauth.ErrPasswordIsTooWeak,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sv := check.NewPassword(16, tt.opts...)

			strength, acceptStatus := sv.CalcStrength(tt.userPassword)
			assert.Equal(t, password.CalcStrength(tt.userPassword).String(), strength)
			assert.Equal(t, tt.wantStatus, acceptStatus)

			// errors.Is с nil-целью истинен только для nil-ошибки, поэтому допустимый пароль проверяется так же
			assert.ErrorIs(t, sv.Check(tt.userPassword), tt.wantErr)
		})
	}
}
