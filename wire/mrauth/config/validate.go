package config

import (
	"fmt"
	"regexp"
	"slices"
	"time"

	localecfg "github.com/mondegor/go-core/mrlocale/config"
	"github.com/mondegor/go-core/util/crypt/password"
	timezonecfg "github.com/mondegor/go-core/util/timezone/config"

	"github.com/mondegor/go-components/mrauth/model/usergroup"
)

const (
	accessTypeJWT     = "jwt"
	accessTypeSession = "session"

	// minHMACSecretHS256 - минимальная длина HMAC-секрета для HS256 (256 бит, RFC 7518 §3.2).
	minHMACSecretHS256 = 32

	// minHMACSecretHS512 - минимальная длина HMAC-секрета для HS512 (512 бит, RFC 7518 §3.2).
	minHMACSecretHS512 = 64

	// defaultTOTPIssuer - имя издателя TOTP по умолчанию (метка в приложении-аутентификаторе).
	defaultTOTPIssuer = "Auth"

	// defaultRecoveryCount - число выдаваемых аварийных кодов по умолчанию.
	defaultRecoveryCount = 10

	// defaultRecoveryCodeLength - длина одного аварийного кода по умолчанию.
	defaultRecoveryCodeLength = 17

	// minRecoveryCodeLength, maxRecoveryCodeLength - границы длины аварийного кода: повторяют
	// умолчания auth2fa.Verifier и ограничение RECOVERY в спеке (поле secret).
	minRecoveryCodeLength = 8
	maxRecoveryCodeLength = 32

	// minConfirmCodeLength, maxConfirmCodeLength - границы длины кода подтверждения
	// операции: ограничение EMAIL/PHONE в спеке (поле secret).
	minConfirmCodeLength = 4
	maxConfirmCodeLength = 8

	// minNewEmailExpiry - порог срока жизни операции подтверждения нового емаила, повторяет
	// secureoperation.fixedExpiryThreshold: срок не больше порога продлевался бы каждой
	// повторной отправкой кода, а по спеке он отсчитывается от создания операции.
	minNewEmailExpiry = time.Hour

	// defaultRecoveryLowThreshold - остаток кодов по умолчанию, при котором слать предупреждение.
	defaultRecoveryLowThreshold = 3

	// defaultDecoyTOTPPercent - доля подставного TOTP по умолчанию: применяется, когда реальное
	// распределение вторых факторов в инсталляции неизвестно.
	defaultDecoyTOTPPercent = 50

	// minDecoyTOTPPercent, maxDecoyTOTPPercent - границы доли подставного TOTP, повторяют
	// инвариант crypt.NewDecoyFactorSelector. Края диапазона исключены намеренно: и 0, и 100
	// раздали бы один и тот же подставной тип всем аккаунтам без 2FA, а вырожденное
	// распределение выдаёт заглушку не хуже расхождения с реальным.
	minDecoyTOTPPercent = 1
	maxDecoyTOTPPercent = 99

	// minDecoyFactorSaltLength - минимальная длина соли подставного второго фактора
	// (256 бит), повторяет инвариант crypt.NewDecoyFactorSelector. Проверки на непустоту
	// недостаточно: короткая соль перебирается, а подобравший её вычисляет ожидаемый
	// подставной тип и по расхождению читает, включена ли у аккаунта 2FA.
	minDecoyFactorSaltLength = 32

	// defaultPasswordMinStrength - минимальная надёжность пароля 2FA по умолчанию.
	defaultPasswordMinStrength = "STRONG"

	// defaultConfirmMaxAttempts - число попыток ввода второго фактора по умолчанию.
	defaultConfirmMaxAttempts = 5

	// defaultConfirmExpiry - срок жизни звена подтверждения вторым фактором по умолчанию.
	defaultConfirmExpiry = 30 * time.Minute

	// maxConfirmExpiry - потолок срока жизни звена подтверждения вторым фактором.
	maxConfirmExpiry = time.Hour

	// minSessionThreshold - нижняя граница soft/hard отклонения от лимита сессий
	// (зеркалит клампинг domain-слоя correctThresholds).
	minSessionThreshold int8 = -4

	// maxSessionThreshold - потолок soft/hard отклонения от лимита сессий
	// (зеркалит клампинг domain-слоя correctThresholds).
	maxSessionThreshold int8 = 16

	// maxStorableTimeZone - предельная длина IANA-имени, пригодного для хранения в колонке
	// user_timezone, равная её ширине (см. _sample/migrations, users.user_timezone varchar(64)).
	// Связь неявная - при изменении ширины колонки константу нужно править вручную.
	//
	// Реальные IANA-имена вдвое короче, поэтому это структурный предохранитель на случай
	// имени-самоделки, а не ограничение выбора: существующий пояс в этот предел укладывается.
	maxStorableTimeZone = 64
)

// regexpStorableLang - формат языка, пригодного для хранения в колонке lang_code:
// двухбуквенный код языка и необязательный двухбуквенный код региона.
//
// Пригодность обеспечивается структурно: длиннее 5 символов такая запись быть не может,
// что и есть ширина колонки (см. _sample/migrations, users.lang_code varchar(5)).
// Связь неявная - при изменении ширины колонки регулярку нужно править вручную.
//
// Намеренно строже, чем validate.Lang: тот проверяет лишь форму записи на границе ввода
// и принимает неканоничные варианты ("ru_RU", "en-us", "rus"). Сюда же код доходит уже
// каноничным (это удерживает mrlocale/config.ValidateLanguages), и пойдёт он прямо
// в колонку, поэтому послабления недопустимы.
var regexpStorableLang = regexp.MustCompile(`^[a-z]{2}(-[A-Z]{2})?$`)

// CorrectValuesAuth2FA - подставляет значения по умолчанию в незаданные поля настроек 2FA.
func CorrectValuesAuth2FA(cfg Auth2FA) Auth2FA {
	if cfg.TOTPIssuer == "" {
		cfg.TOTPIssuer = defaultTOTPIssuer
	}

	if cfg.RecoveryCount < 1 {
		cfg.RecoveryCount = defaultRecoveryCount
	}

	if cfg.RecoveryCodeLength < 1 {
		cfg.RecoveryCodeLength = defaultRecoveryCodeLength
	}

	if cfg.RecoveryLowThreshold < 1 {
		cfg.RecoveryLowThreshold = defaultRecoveryLowThreshold
	}

	if cfg.DecoyTOTPPercent < minDecoyTOTPPercent || cfg.DecoyTOTPPercent > maxDecoyTOTPPercent {
		cfg.DecoyTOTPPercent = defaultDecoyTOTPPercent
	}

	if cfg.ConfirmMaxAttempts < 1 {
		cfg.ConfirmMaxAttempts = defaultConfirmMaxAttempts
	}

	// потолок задан затем, что сроки звеньев складываются (см. maxConfirmExpiry)
	if cfg.ConfirmExpiry < 1 || cfg.ConfirmExpiry > maxConfirmExpiry {
		cfg.ConfirmExpiry = defaultConfirmExpiry
	}

	if cfg.PasswordMinStrength == "" {
		cfg.PasswordMinStrength = defaultPasswordMinStrength
	}

	return cfg
}

// ParsePasswordStrength - разбирает имя уровня надёжности пароля (WEAK, MIDDLE, STRONG,
// THE_BEST - шкала password.CalcStrength). NOT_RATED и неизвестное имя - ошибка: пароль
// без оценки не может быть допустимым вторым фактором.
func ParsePasswordStrength(name string) (password.PassStrength, error) {
	for value := password.PassStrengthWeak; value <= password.PassStrengthBest; value++ {
		if value.String() == name {
			return value, nil
		}
	}

	return password.PassStrengthNotRated, fmt.Errorf("password strength '%s' is unknown", name)
}

// ValidateAuth2FA - проверяет настройки 2FA, которые нельзя привести к допустимым
// значениям подстановкой умолчаний (см. CorrectValuesAuth2FA): соль подставного второго
// фактора - секрет инсталляции, и взять его неоткуда, поэтому короткая соль остаётся ошибкой;
// неизвестное имя порога надёжности пароля 2FA и длина аварийного кода вне допустимого
// диапазона - тоже ошибки, а не повод молча взять умолчание.
//
// Без этой проверки короткая соль роняет старт из глубины сборки контроллера
// (crypt.NewDecoyFactorSelector), то есть из места, где конфигурация уже считается проверенной.
//
// Это host-only reference-валидация уровня composition-root: предполагается, что её вызывает
// host-приложение из своего init-пути (внутри библиотеки она намеренно не вызывается). Конкретный
// проект может использовать её как есть либо написать собственную.
func ValidateAuth2FA(cfg Auth2FA) error {
	// значение соли в ошибку не попадает: это секрет, а логи ошибок старта обычно не защищены
	if len(cfg.DecoyFactorSalt) < minDecoyFactorSaltLength {
		return fmt.Errorf(
			"decoy factor salt is too short (got %d bytes, min=%d)",
			len(cfg.DecoyFactorSalt),
			minDecoyFactorSaltLength,
		)
	}

	// незаданный порог допустим: его подставляет CorrectValuesAuth2FA
	if cfg.PasswordMinStrength != "" {
		if _, err := ParsePasswordStrength(cfg.PasswordMinStrength); err != nil {
			return err
		}
	}

	// незаданная длина допустима: её подставляет CorrectValuesAuth2FA
	if cfg.RecoveryCodeLength != 0 &&
		(cfg.RecoveryCodeLength < minRecoveryCodeLength || cfg.RecoveryCodeLength > maxRecoveryCodeLength) {
		return fmt.Errorf(
			"recovery code length is out of range (got %d, min=%d, max=%d)",
			cfg.RecoveryCodeLength,
			minRecoveryCodeLength,
			maxRecoveryCodeLength,
		)
	}

	return nil
}

// ValidateOperationConfirm - проверяет настройки подтверждения операций на соответствие спеке:
// длина кода подтверждения в допустимом диапазоне и срок жизни операции подтверждения нового
// емаила больше порога фиксированного срока (незаданный срок допустим - фабрика подставит
// умолчание).
//
// Это host-only reference-валидация уровня composition-root: предполагается, что её вызывает
// host-приложение из своего init-пути (внутри библиотеки она намеренно не вызывается). Конкретный
// проект может использовать её как есть либо написать собственную.
func ValidateOperationConfirm(cfg OperationConfirm) error {
	if err := validateConfirmCodeLength(cfg.CodeLength); err != nil {
		return err
	}

	if cfg.NewEmailExpiry != 0 && cfg.NewEmailExpiry <= minNewEmailExpiry {
		return fmt.Errorf(
			"new email expiry must be greater than %s (got %s)",
			minNewEmailExpiry,
			cfg.NewEmailExpiry,
		)
	}

	return nil
}

func validateConfirmCodeLength(length uint8) error {
	if length < minConfirmCodeLength || length > maxConfirmCodeLength {
		return fmt.Errorf(
			"confirm code length is out of range (got %d, min=%d, max=%d)",
			length,
			minConfirmCodeLength,
			maxConfirmCodeLength,
		)
	}

	return nil
}

// ValidateRealms - проверяет конфигурацию realm'ов: уникальность id и имён, корректность типов токенов,
// TTL jwt-токенов, принадлежность ролей известному набору, допустимость имён видов пользователей
// (без '/' - см. ограничение в описании UserRealm) и длину кода подтверждения, если она задана.
func ValidateRealms(realms []UserRealm, allRoles []string) error {
	uniqRealms := make(map[string]bool, len(realms))
	uniqRealmIDs := make(map[uint16]bool, len(realms))

	for _, realm := range realms {
		if realm.ID == 0 {
			return fmt.Errorf("realm id is empty for realm '%s'", realm.Name)
		}

		if uniqRealmIDs[realm.ID] {
			return fmt.Errorf("duplicate realm id '%d'", realm.ID)
		}

		if uniqRealms[realm.Name] {
			return fmt.Errorf("duplicate realm name '%s'", realm.Name)
		}

		if realm.RegisterUserKind == "" {
			return fmt.Errorf("registerUser is empty for realm '%s'", realm.Name)
		}

		if realm.AuthToken.AccessType != accessTypeJWT && realm.AuthToken.AccessType != accessTypeSession {
			return fmt.Errorf("invalid token type for realm (type='%s', realm='%s')", realm.AuthToken.AccessType, realm.Name)
		}

		uniqRealmIDs[realm.ID] = true
		uniqRealms[realm.Name] = true

		if err := validateRealm(realm, allRoles); err != nil {
			return err
		}
	}

	return nil
}

func validateRealm(realm UserRealm, allRoles []string) error {
	uniqKinds := make(map[string]bool, len(realm.UserKinds))
	hasRegisterUser := realm.RegisterUserKind == "none"

	for _, kind := range realm.UserKinds {
		// '/' в имени вида ломает разбор группы "{realm}/{kind}" и молча теряет per-realm
		// статистику, поэтому отвергается на старте (см. ограничение в описании UserRealm)
		if err := usergroup.ValidateKind(kind.Name); err != nil {
			return fmt.Errorf("invalid user kind name for realm (kind='%s', realm='%s'): %w", kind.Name, realm.Name, err)
		}

		if uniqKinds[kind.Name] {
			return fmt.Errorf("duplicate user kind name for realm (kind='%s', realm='%s')", kind.Name, realm.Name)
		}

		uniqKinds[kind.Name] = true

		if realm.RegisterUserKind == kind.Name {
			hasRegisterUser = true
		}

		for _, role := range kind.Roles {
			if !slices.Contains(allRoles, role) {
				return fmt.Errorf("role of user kind is not found in roles for realm (role='%s', kind='%s', realm='%s')", role, kind.Name, realm.Name)
			}
		}
	}

	if !hasRegisterUser {
		return fmt.Errorf("realm.RegisterUserKind is not found in realm.UserKinds for realm (kind='%s', realm='%s')", realm.RegisterUserKind, realm.Name)
	}

	// незаданная длина допустима: её подставляет CorrectValuesRealm
	if realm.OperationConfirm.CodeLength != 0 {
		if err := validateConfirmCodeLength(realm.OperationConfirm.CodeLength); err != nil {
			return fmt.Errorf("invalid operation confirm for realm '%s': %w", realm.Name, err)
		}
	}

	return nil
}

// CorrectValuesRealm - подставляет значения по умолчанию в незаданные поля realm'ов
// и применяет override параметров токена.
func CorrectValuesRealm(realms []UserRealm, defaultConfirm OperationConfirm, overrideToken Token) []UserRealm {
	for i := range realms {
		rop := &realms[i].OperationConfirm

		if rop.TokenLength < 1 {
			rop.TokenLength = defaultConfirm.TokenLength
		}

		if rop.CodeLength < 1 {
			rop.CodeLength = defaultConfirm.CodeLength
		}

		if rop.SessionExpiry < 1 {
			rop.SessionExpiry = defaultConfirm.SessionExpiry
		}

		rop.SendByEmail = correctValuesCodeSender(rop.SendByEmail, defaultConfirm.SendByEmail)
		rop.SendByPhone = correctValuesCodeSender(rop.SendByPhone, defaultConfirm.SendByPhone)

		rt := &realms[i].AuthToken

		if overrideToken.AccessType != "" {
			rt.AccessType = overrideToken.AccessType
		}

		if overrideToken.AccessExpiry != 0 {
			rt.AccessExpiry = overrideToken.AccessExpiry
		}

		if overrideToken.RefreshExpiry != 0 {
			rt.RefreshExpiry = overrideToken.RefreshExpiry
		}
	}

	return realms
}

func correctValuesCodeSender(cs, defaultSender CodeSender) CodeSender {
	if cs.MaxAttempts < 1 {
		cs.MaxAttempts = defaultSender.MaxAttempts
	}

	if cs.MaxResends < 1 {
		cs.MaxResends = defaultSender.MaxResends
	}

	if cs.MinResendTime < 1 {
		cs.MinResendTime = defaultSender.MinResendTime
	}

	return cs
}

// IsJWTUsed - сообщает, использует ли хотя бы один realm access_type=jwt.
// Если возвращает false, модуль работает в session-only режиме и InitJWT вызывать не нужно.
func IsJWTUsed(realms []UserRealm) bool {
	for _, realm := range realms {
		if realm.AuthToken.AccessType == accessTypeJWT {
			return true
		}
	}

	return false
}

// ValidateSessionThresholds - проверяет отклонения soft/hard от лимита сессий, которые хост
// передаёт в модуль: оба должны лежать в диапазоне [minSessionThreshold, maxSessionThreshold]
// и hard >= soft.
//
// Это host-only reference-валидация уровня composition-root: предполагается, что её вызывает
// host-приложение из своего init-пути (внутри библиотеки она намеренно не вызывается). Конкретный
// проект может использовать её как есть либо написать собственную.
func ValidateSessionThresholds(soft, hard int8) error {
	if soft < minSessionThreshold || hard < minSessionThreshold {
		return fmt.Errorf("session threshold below min (got soft=%d hard=%d, min=%d)", soft, hard, minSessionThreshold)
	}

	if soft > maxSessionThreshold || hard > maxSessionThreshold {
		return fmt.Errorf("session threshold exceeds max (got soft=%d hard=%d, max=%d)", soft, hard, maxSessionThreshold)
	}

	if hard < soft {
		return fmt.Errorf("session hard threshold must be >= soft (got soft=%d hard=%d)", soft, hard)
	}

	return nil
}

// ValidateLanguages - проверяет, что языки приложения пригодны для хранения
// в профиле пользователя: сверх общих требований к списку (см. mrlocale/config.ValidateLanguages,
// она вызывается здесь же) каждый язык должен укладываться в формат колонки lang_code.
//
// В профиль попадает язык из этого же списка (на границе ввода принимается только он,
// см. TagLang), поэтому ограничение колонки удерживается целиком здесь: язык со скриптовым
// сабтегом ("zh-Hans"), трёхбуквенным кодом ("fil") или регионом-числом ("es-419") в колонку
// не помещается и уронил бы запись уже в рантайме. Для самого mrlocale это законные языки,
// поэтому отвергнуть их может только тот, кто знает про колонку.
//
// Это host-only reference-валидация уровня composition-root: предполагается, что её вызывает
// host-приложение из своего init-пути (внутри библиотеки она намеренно не вызывается). Конкретный
// проект может использовать её как есть либо написать собственную.
func ValidateLanguages(langs []string) error {
	if err := localecfg.ValidateLanguages(langs); err != nil {
		return err
	}

	// после проверки выше запись каждого языка каноническая,
	// поэтому в колонку пойдёт ровно эта строка
	for _, lang := range langs {
		if !regexpStorableLang.MatchString(lang) {
			return fmt.Errorf(
				"language '%s' is not storable: expected 2-letter code with optional 2-letter region (e.g. ru, ru-RU)",
				lang,
			)
		}
	}

	return nil
}

// ValidateTimeZones - проверяет, что часовые пояса приложения пригодны для хранения
// в профиле пользователя: сверх общих требований к списку (см. util/timezone/config.ValidateTimeZones,
// она вызывается здесь же) каждое имя должно укладываться в ширину колонки user_timezone.
//
// Парная к ValidateLanguages и нужна по той же причине: в профиль попадает пояс
// из этого же списка (на границе ввода принимается только он, см. TagTimeZone). Для самого
// timezone.LocationList длина имени безразлична, поэтому упереться в колонку может только тот,
// кто про неё знает.
//
// Это host-only reference-валидация уровня composition-root: предполагается, что её вызывает
// host-приложение из своего init-пути (внутри библиотеки она намеренно не вызывается). Конкретный
// проект может использовать её как есть либо написать собственную.
func ValidateTimeZones(zones []string) error {
	if err := timezonecfg.ValidateTimeZones(zones); err != nil {
		return err
	}

	for _, zone := range zones {
		if len(zone) > maxStorableTimeZone {
			return fmt.Errorf(
				"timezone '%s' is not storable: expected at most %d characters",
				zone,
				maxStorableTimeZone,
			)
		}
	}

	return nil
}
