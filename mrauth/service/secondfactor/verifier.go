package secondfactor

import (
	"context"

	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
)

const (
	defaultMinRecoveryCodeLength = 8
	defaultMaxRecoveryCodeLength = 32
)

type (
	// Verifier - проверка второго фактора (TOTP с fallback на аварийный код, либо пароль).
	Verifier struct {
		storage               user2faSource
		codeComparer          codeComparer
		totpValidator         totpValidator
		minRecoveryCodeLength int
		maxRecoveryCodeLength int
	}

	user2faSource interface {
		FetchOne(ctx context.Context, userID uuid.UUID) (entity.Auth2fa, error)
		UpdateRecoveryCode(ctx context.Context, userID uuid.UUID, hash string) error
		UpdateTOTPStep(ctx context.Context, userID uuid.UUID, step int64) error
	}

	codeComparer interface {
		CompareSecretAndHash(secret, hashedSecret string) (ok bool, err error)
	}

	totpValidator interface {
		ValidateCode(code, secret string) (ok bool, timeStep int64, err error)
	}
)

// NewVerifier - создаёт объект Verifier.
func NewVerifier(
	storage user2faSource,
	comparer codeComparer,
	totpValidator totpValidator,
	minRecoveryCodeLength int,
	maxRecoveryCodeLength int,
) *Verifier {
	if minRecoveryCodeLength < 1 {
		minRecoveryCodeLength = defaultMinRecoveryCodeLength
	}

	if maxRecoveryCodeLength < 1 {
		maxRecoveryCodeLength = defaultMaxRecoveryCodeLength
	}

	if minRecoveryCodeLength > maxRecoveryCodeLength {
		maxRecoveryCodeLength = minRecoveryCodeLength
	}

	return &Verifier{
		storage:               storage,
		codeComparer:          comparer,
		totpValidator:         totpValidator,
		minRecoveryCodeLength: minRecoveryCodeLength,
		maxRecoveryCodeLength: maxRecoveryCodeLength,
	}
}

// Verify - проверяет code как второй фактор. Для TOTP при срабатывании аварийного
// кода возвращает commit, который должен быть вызван в транзакции подтверждения.
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

		ok, err = v.codeComparer.CompareSecretAndHash(code, row.Secret)
		if err != nil {
			return false, nil, err
		}

		return ok, nil, nil

	case confirmmethod.TOTP:
		if row.Type != auth2fatype.TOTP {
			return false, nil, nil
		}

		ok, timeStep, err := v.totpValidator.ValidateCode(code, row.Secret)
		if err != nil {
			return false, nil, err
		}

		if ok {
			// защита от replay: код математически верен, но его time-step уже был
			// использован ранее - повторное предъявление отклоняется
			if timeStep <= row.LastTOTPStep {
				return false, nil, nil
			}

			commit = func(ctx context.Context) error {
				return v.storage.UpdateTOTPStep(ctx, userID, timeStep)
			}

			return true, commit, nil
		}

		// не проверяем любые коды не похожие на аварийный код
		if !v.looksLikeRecoveryCode(code) {
			return false, nil, nil
		}

		return v.tryRecovery(userID, row.RecoveryCodes, code)

	default:
		return false, nil, nil
	}
}

func (v *Verifier) looksLikeRecoveryCode(s string) bool {
	return len(s) >= v.minRecoveryCodeLength && len(s) <= v.maxRecoveryCodeLength
}

// tryRecovery - перебирает аварийные коды и при совпадении возвращает commit,
// атомарно расходующий израсходованный хеш в транзакции подтверждения.
// Стоимость перебора bcrypt ограничена: число аварийных кодов невелико (recoveryCount),
// а число попыток на операцию лимитировано и сериализовано блокировкой строки операции.
func (v *Verifier) tryRecovery(
	userID uuid.UUID,
	hashes []string,
	code string,
) (bool, func(ctx context.Context) error, error) {
	for _, hash := range hashes {
		ok, err := v.codeComparer.CompareSecretAndHash(code, hash)
		if err != nil {
			return false, nil, err
		}

		if !ok {
			continue
		}

		commit := func(ctx context.Context) error {
			return v.storage.UpdateRecoveryCode(ctx, userID, hash)
		}

		return true, commit, nil
	}

	return false, nil, nil
}
