package auth2fa

type (
	// Option - настройка объекта Verifier.
	Option func(o *options)

	options struct {
		verifier *Verifier
	}
)

// WithRecoveryCodeLength - задаёт границы длины строки, принимаемой как аварийный код
// (вне этого диапазона bcrypt-перебор по аварийным кодам не запускается).
func WithRecoveryCodeLength(minLength, maxLength int) Option {
	return func(o *options) {
		o.verifier.minRecoveryCodeLength = minLength
		o.verifier.maxRecoveryCodeLength = maxLength
	}
}

// WithRecoveryAlerter - подключает оповещение о каждом расходе аварийного кода с остатком.
func WithRecoveryAlerter(alerter recoveryAlerter) Option {
	return func(o *options) {
		o.verifier.recoveryAlerter = alerter
	}
}

// WithDecoySecrets - задаёт подставные секреты, с которыми сверяется доказательство аккаунта
// без 2FA, чтобы время ответа не выдавало его состояние.
func WithDecoySecrets(passwordHash, totpSecret string) Option {
	return func(o *options) {
		o.verifier.decoyPasswordHash = passwordHash
		o.verifier.decoyTOTPSecret = totpSecret
	}
}
