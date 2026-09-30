package auth2fa

import (
	"context"

	"github.com/google/uuid"
	"github.com/mondegor/go-core/errors"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
)

const (
	defaultMinRecoveryCodeLength = 8
	defaultMaxRecoveryCodeLength = 32

	// defaultDecoyPasswordHash - хеш в формате и стоимости, которые принимает comparer, подключаемый
	// в wire (crypt.SecretGenerator: bcrypt, cost 10); при другом comparer задаётся через
	// WithDecoySecrets. Хеш случайного секрета, который никому не известен и нигде не хранится.
	// Сверка с ним заведомо не сходится и нужна только затем, чтобы аккаунт без 2FA отвечал
	// столько же времени, сколько аккаунт с 2FA и неверным паролем (см. verifyDecoy). Значение
	// открытое намеренно: доступа оно не даёт ни при каком коде, важна только стоимость сверки с ним.
	defaultDecoyPasswordHash = "$2a$10$zIPrpypLwJzdjoSwKwjBGe8lsL03rYvSWJ/EKTgXKOpN6imDwcGTW" //nolint:gosec // не секрет, см. выше

	// defaultDecoyTOTPSecret - корректный base32-секрет для той же цели, что и
	// defaultDecoyPasswordHash: подставляется, когда сверять код не с чем.
	defaultDecoyTOTPSecret = "JBSWY3DPEHPK3PXP" //nolint:gosec // не секрет, см. defaultDecoyPasswordHash
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
		decoyPasswordHash     string
		decoyTOTPSecret       string
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
	// в очередь, а не слать письмо синхронно), иначе транзакция подтверждения удерживается
	// дольше нужного.
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
			decoyPasswordHash:     defaultDecoyPasswordHash,
			decoyTOTPSecret:       defaultDecoyTOTPSecret,
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

	// пустые подставные секреты сделали бы сверку мгновенной и выдали бы аккаунт без 2FA
	if o.verifier.decoyPasswordHash == "" {
		o.verifier.decoyPasswordHash = defaultDecoyPasswordHash
	}

	if o.verifier.decoyTOTPSecret == "" {
		o.verifier.decoyTOTPSecret = defaultDecoyTOTPSecret
	}

	return o.verifier
}

// Verify - проверяет code как второй фактор (пароль или TOTP). Если основное доказательство
// не сошлось и для текущего действия операции допустим аварийный код (allowRecovery), то code
// дополнительно проверяется как аварийный. Метод confirmmethod.Recovery проверяет code как
// доказательство завершающего звена цепочки: там аварийный код и есть само доказательство,
// поэтому признак allowRecovery для него не нужен.
// При расходе аварийного кода или продвижении TOTP-шага возвращает commit, который должен
// быть вызван в транзакции подтверждения.
//
// Отсутствие записи 2FA отдаётся как mrauth.ErrAuth2FAIsDisabled - это факт, известный только
// хранилищу. Скрывать ли его за неверным доказательством, решает вызывающий: подтверждение
// операции скрывает, потому что метод гостевой. Стоимость сверки оплачивается здесь в любом
// случае, потому что решение вызывающего на неё не влияет.
func (v *Verifier) Verify(
	ctx context.Context,
	userID uuid.UUID,
	method confirmmethod.Enum,
	allowRecovery bool,
	code string,
) (ok bool, commit func(ctx context.Context) error, err error) {
	row, err := v.storage.FetchOne(ctx, userID)
	if err != nil {
		if !errors.Is(err, errors.ErrEventStorageNoRecordFound) {
			return false, nil, err
		}

		// записи 2FA нет по одной из двух причин: либо у аккаунта её никогда не было
		// (цепочка-заглушка входа по аварийному коду), либо её удалили между созданием операции
		// и её подтверждением (в том числе между подтверждением предыдущего звена
		// и предъявлением аварийного кода). Различить их здесь нечем, поэтому сверка
		// с подставным секретом делается до возврата в обеих: там, где ответ будет выдан
		// за неверное доказательство, мгновенный ответ выдал бы состояние аккаунта
		v.verifyDecoy(method, code)

		return false, nil, mrauth.ErrAuth2FAIsDisabled
	}

	// аварийный код сверяется в двух случаях: он допустим вместо основного доказательства текущего
	// действия (allowRecovery) либо он сам является доказательством завершающего звена цепочки
	// (confirmmethod.Recovery); в остальных случаях хеши не сравниваются вовсе и код не расходуется
	skipRecovery := !allowRecovery && method != confirmmethod.Recovery

	switch method {
	case confirmmethod.Password:
		if row.Type != auth2fatype.Password {
			// тип второго фактора сменился после создания операции: основного доказательства
			// у звена больше нет, но аварийный код от типа не зависит - он сверяется ниже,
			// и терять его из-за смены типа звено не должно; сверка с подставным секретом
			// уравнивает время ответа с ответом на неверное значение фактора
			v.verifyDecoy(method, code)

			break
		}

		ok, err := v.passwordComparer.CompareSecretAndHash(code, row.Secret)
		if err != nil {
			return false, nil, err
		}

		if ok {
			return true, nil, nil
		}
	case confirmmethod.TOTP:
		if row.Type != auth2fatype.TOTP {
			// тип сменился, см. пояснение в ветке confirmmethod.Password
			v.verifyDecoy(method, code)

			break
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
				if err := v.storage.UpdateTOTPStep(ctx, userID, timeStep); err != nil {
					// шаг не продвинулся: тот же time-step уже израсходован конкурентным
					// подтверждением либо записи 2FA больше нет - по ошибке хранилища эти случаи
					// не различить, и на исход это не влияет
					if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
						return mrauth.ErrEventAuth2FACodeAlreadyUsed
					}

					return err
				}

				return nil
			}

			return true, commit, nil
		}
	case confirmmethod.Recovery:
		// основного доказательства у этого звена нет: сам аварийный код и есть доказательство,
		// поэтому проверка выполняется ниже, если предъявлен действительно аварийный код
	default:
		return false, nil, nil
	}

	// основное доказательство не сошлось либо его у звена и не было (confirmmethod.Recovery)
	if skipRecovery || !v.looksLikeRecoveryCode(code) {
		return false, nil, nil
	}

	return v.tryRecovery(userID, row.RecoveryCodes, code)
}

// verifyDecoy - сверяет code с подставным секретом и отбрасывает исход сверки. Нужна ровно
// затем, чтобы аккаунт без 2FA отвечал столько же времени, сколько аккаунт с 2FA и неверным
// доказательством: без сверки ответ пришёл бы мгновенно и выдал бы состояние аккаунта.
// Ошибка сверки тоже отбрасывается - подставной секрет заведомо корректен, а всплывшая наружу
// ошибка была бы тем самым признаком, который метод и скрывает.
//
// Покрыты только Password и TOTP: confirmmethod.Recovery намеренно оставлен без подстановки,
// потому что скрывать на нём нечего. Аварийный код - завершающее звено цепочки, и доходит
// до него только тот, кто уже подтвердил второй фактор, то есть про включённую 2FA он и так
// знает; на подставной цепочке аккаунта без 2FA это звено недостижимо вовсе, ведь первое
// её звено не подтверждается никаким кодом.
func (v *Verifier) verifyDecoy(method confirmmethod.Enum, code string) {
	switch method {
	case confirmmethod.Password:
		_, _ = v.passwordComparer.CompareSecretAndHash(code, v.decoyPasswordHash)
	case confirmmethod.TOTP:
		_, _, _ = v.totpValidator.ValidateCode(code, v.decoyTOTPSecret)
	}
}

func (v *Verifier) looksLikeRecoveryCode(s string) bool {
	return len(s) >= v.minRecoveryCodeLength && len(s) <= v.maxRecoveryCodeLength
}

// tryRecovery - перебирает аварийные коды и при совпадении возвращает commit,
// атомарно расходующий израсходованный хеш в транзакции подтверждения.
// Стоимость перебора хешей ограничена: число аварийных кодов невелико (recoveryCount),
// а число попыток на операцию лимитировано, и попытки одной операции не выполняются параллельно.
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
				// хеша в наборе уже нет: код израсходован конкурентным подтверждением
				if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
					return mrauth.ErrEventAuth2FACodeAlreadyUsed
				}

				return err
			}

			// оповещается о каждом расходе вместе с остатком аварийных кодов;
			// вызов идёт в той же транзакции подтверждения
			return v.recoveryAlerter.SendAlert(ctx, userID, remaining)
		}

		return true, commit, nil
	}

	return false, nil, nil
}

type (
	// defaultRecoveryAlerter - заглушка alerter'а по умолчанию.
	defaultRecoveryAlerter struct{}
)

// SendAlert - no-op реализация по умолчанию.
func (defaultRecoveryAlerter) SendAlert(_ context.Context, _ uuid.UUID, _ int) error {
	return nil
}
