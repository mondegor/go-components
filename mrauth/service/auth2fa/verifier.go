package auth2fa

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
	// Verifier - проверка второго фактора
	// (TOTP или пароль, оба с fallback на аварийный код).
	Verifier struct {
		storage               user2faSource
		passwordComparer      passwordComparer
		totpValidator         totpValidator
		recoveryAlerter       recoveryAlerter // OPTIONAL
		minRecoveryCodeLength int
		maxRecoveryCodeLength int
	}

	user2faSource interface {
		FetchOne(ctx context.Context, userID uuid.UUID) (entity.Auth2FA, error)
		UpdateRecoveryCode(ctx context.Context, userID uuid.UUID, hash string) (remaining int, err error)
		UpdateTOTPStep(ctx context.Context, userID uuid.UUID, step int64) error
	}

	passwordComparer interface {
		CompareSecretAndHash(secret, hashedSecret string) (ok bool, err error)
	}

	totpValidator interface {
		ValidateCode(code, secret string) (ok bool, timeStep int64, err error)
	}

	// recoveryAlerter - оповещает о расходе аварийного кода. SendAlert вызывается из commit
	// внутри транзакции подтверждения на каждый израсходованный код, поэтому реализация
	// обязана быть дешёвой и не выполнять блокирующий сетевой IO (например, ставить задачу
	// в очередь, а не слать письмо синхронно), иначе соединение БД и блокировка строки
	// удерживаются дольше нужного.
	recoveryAlerter interface {
		SendAlert(ctx context.Context, userID uuid.UUID, codeRemaining int) error
	}
)

// NewVerifier - создаёт объект Verifier.
func NewVerifier(
	storage user2faSource,
	passwordComparer passwordComparer,
	totpValidator totpValidator,
	opts ...Option,
) *Verifier {
	o := options{
		verifier: &Verifier{
			storage:               storage,
			passwordComparer:      passwordComparer,
			totpValidator:         totpValidator,
			minRecoveryCodeLength: defaultMinRecoveryCodeLength,
			maxRecoveryCodeLength: defaultMaxRecoveryCodeLength,
		},
	}

	for _, opt := range opts {
		opt(&o)
	}

	if o.verifier.recoveryAlerter == nil {
		o.verifier.recoveryAlerter = defaultRecoveryAlerter{}
	}

	// нормализация границ длины после применения опций
	if o.verifier.minRecoveryCodeLength < 1 {
		o.verifier.minRecoveryCodeLength = defaultMinRecoveryCodeLength
	}

	if o.verifier.maxRecoveryCodeLength < 1 {
		o.verifier.maxRecoveryCodeLength = defaultMaxRecoveryCodeLength
	}

	if o.verifier.minRecoveryCodeLength > o.verifier.maxRecoveryCodeLength {
		o.verifier.maxRecoveryCodeLength = o.verifier.minRecoveryCodeLength
	}

	return o.verifier
}

// Verify - проверяет code как второй фактор. Для TOTP и пароля при срабатывании
// аварийного кода возвращает commit, который должен быть вызван в транзакции подтверждения.
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

		ok, err = v.passwordComparer.CompareSecretAndHash(code, row.Secret)
		if err != nil {
			return false, nil, err
		}

		if ok {
			return true, nil, nil
		}
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
	default:
		return false, nil, nil
	}

	// не проверяем любые коды не похожие на аварийный код
	if !v.looksLikeRecoveryCode(code) {
		return false, nil, nil
	}

	return v.tryRecovery(userID, row.RecoveryCodes, code)
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
		ok, err := v.passwordComparer.CompareSecretAndHash(code, hash)
		if err != nil {
			return false, nil, err
		}

		if !ok {
			continue
		}

		commit := func(ctx context.Context) error {
			remaining, err := v.storage.UpdateRecoveryCode(ctx, userID, hash)
			if err != nil {
				return err
			}

			// сообщаем остаток аварийных кодов; нужно ли уведомлять пользователя
			// (порог) - решает alerter. Вызов идёт в той же транзакции подтверждения.
			return v.recoveryAlerter.SendAlert(ctx, userID, remaining)
		}

		return true, commit, nil
	}

	return false, nil, nil
}

type (
	// defaultRecoveryAlerter - заглушка alerter'а по умолчанию (ничего не делает),
	// чтобы поле recoveryAlerter всегда было задано и не требовало проверки на nil.
	defaultRecoveryAlerter struct{}
)

// SendAlert - no-op реализация по умолчанию.
func (defaultRecoveryAlerter) SendAlert(_ context.Context, _ uuid.UUID, _ int) error {
	return nil
}
