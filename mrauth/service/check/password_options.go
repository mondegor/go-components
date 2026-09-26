package check

import (
	"github.com/mondegor/go-core/util/crypt/password"
)

const (
	// defaultMinStrength - минимальная надёжность пароля 2FA по умолчанию.
	defaultMinStrength = password.PassStrengthStrong
)

type (
	// PasswordOption - настройка объекта Password.
	PasswordOption func(o *passwordOptions)

	passwordOptions struct {
		minStrength password.PassStrength
	}
)

// WithMinStrength - задаёт минимальную надёжность пароля, устанавливаемого как 2FA
// (по оценке password.CalcStrength); PassStrengthNotRated означает значение по умолчанию.
func WithMinStrength(value password.PassStrength) PasswordOption {
	return func(o *passwordOptions) {
		o.minStrength = value
	}
}

func newPasswordOptions(opts []PasswordOption) passwordOptions {
	o := passwordOptions{minStrength: defaultMinStrength}

	for _, opt := range opts {
		opt(&o)
	}

	// снять проверку целиком нельзя: пароль без оценки не бывает допустимым вторым фактором
	if o.minStrength == password.PassStrengthNotRated {
		o.minStrength = defaultMinStrength
	}

	return o
}
