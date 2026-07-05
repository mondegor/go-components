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

// WithRecoveryAlerter - подключает уведомление об остатке аварийных кодов после расхода
// (порог, при котором реально слать уведомление, определяет сам alerter).
func WithRecoveryAlerter(alerter recoveryAlerter) Option {
	return func(o *options) {
		o.verifier.recoveryAlerter = alerter
	}
}
