package secondfactor

import (
	"context"

	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
)

type (
	// Verifier - проверка второго фактора (TOTP с fallback на аварийный код, либо пароль).
	Verifier struct {
		storage       user2faSource
		codeComparer  codeComparer
		totpValidator totpValidator
	}

	user2faSource interface {
		FetchOne(ctx context.Context, userID uuid.UUID) (entity.Auth2fa, error)
		ConsumeRecoveryCode(ctx context.Context, userID uuid.UUID, hash string) error
	}

	codeComparer interface {
		CompareSecretAndHash(secret, hashedSecret string) error
	}

	totpValidator interface {
		Validate(code, secret string) bool
	}
)

// NewVerifier - создаёт объект Verifier.
func NewVerifier(storage user2faSource, comparer codeComparer, totpValidator totpValidator) *Verifier {
	return &Verifier{
		storage:       storage,
		codeComparer:  comparer,
		totpValidator: totpValidator,
	}
}

// Verify - проверяет code как второй фактор. Для TOTP при срабатывании аварийного
// кода возвращает commit, который должен быть вызван в транзакции подтверждения.
// TODO: можно сделать два метода Verify и VerifyWithCommit.
func (v *Verifier) Verify(
	ctx context.Context,
	userID uuid.UUID,
	method confirmmethod.Enum,
	code string,
) (ok bool, commit func(ctx context.Context) error, err error) {
	row, err := v.storage.FetchOne(ctx, userID)
	if err != nil {
		return false, nil, err
	}

	switch method {
	case confirmmethod.Password:
		if row.Type != auth2fatype.Password {
			return false, nil, nil
		}

		return v.codeComparer.CompareSecretAndHash(code, row.Secret) == nil, nil, nil

	case confirmmethod.TOTP:
		if row.Type != auth2fatype.TOTP {
			return false, nil, nil
		}

		if v.totpValidator.Validate(code, row.Secret) {
			return true, nil, nil
		}

		// TOTP-код (RFC 6238) всегда состоит из цифр, аварийный код - нет;
		// не перебираем bcrypt-хеши, если введён код в формате TOTP.
		if isAllDigits(code) {
			return false, nil, nil
		}

		return v.tryRecovery(userID, row.RecoveryCodes, code)

	default:
		return false, nil, nil
	}
}

// tryRecovery - перебирает аварийные коды и при совпадении возвращает commit,
// атомарно расходующий израсходованный хеш в транзакции подтверждения.
func (v *Verifier) tryRecovery(
	userID uuid.UUID,
	hashes []string,
	code string,
) (bool, func(ctx context.Context) error, error) {
	for _, hash := range hashes {
		if v.codeComparer.CompareSecretAndHash(code, hash) != nil {
			continue
		}

		commit := func(ctx context.Context) error {
			return v.storage.ConsumeRecoveryCode(ctx, userID, hash)
		}

		return true, commit, nil
	}

	return false, nil, nil
}

// isAllDigits - сообщает, состоит ли строка только из ASCII-цифр (формат TOTP-кода).
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}

	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}
