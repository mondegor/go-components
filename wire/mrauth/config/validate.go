package config

import (
	"fmt"
	"regexp"
	"slices"

	"golang.org/x/text/language"

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

	// defaultRecoveryLowThreshold - остаток кодов по умолчанию, при котором слать предупреждение.
	defaultRecoveryLowThreshold = 2

	// minSessionThreshold - нижняя граница soft/hard отклонения от лимита сессий
	// (зеркалит клампинг domain-слоя correctThresholds).
	minSessionThreshold int8 = -4

	// maxSessionThreshold - потолок soft/hard отклонения от лимита сессий
	// (зеркалит клампинг domain-слоя correctThresholds).
	maxSessionThreshold int8 = 16
)

// regexpStorableLang - формат языка, пригодного для хранения в колонке lang_code:
// двухбуквенный код языка и необязательный двухбуквенный код региона.
//
// Пригодность обеспечивается структурно: длиннее 5 символов такая запись быть не может,
// что и есть ширина колонки (см. _sample/migrations, users.lang_code varchar(5)).
// Связь неявная - при изменении ширины колонки регулярку нужно править вручную.
//
// Намеренно строже, чем validate.Lang: тот проверяет форму записи на границе ввода и
// принимает неканоничные варианты ("ru_RU", "en-us", "rus"), т.к. их приводит к единому виду
// резолвер. Здесь же проверяется уже канонический результат разбора, который пойдёт в колонку,
// поэтому послабления недопустимы.
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

	return cfg
}

// ValidateRealms - проверяет конфигурацию realm'ов: уникальность id и имён, корректность типов токенов,
// TTL jwt-токенов, принадлежность ролей известному набору и допустимость имён видов пользователей
// (без '/' - см. ограничение в описании UserRealm).
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

// ValidateLanguages - проверяет, что языки приложения пригодны для хранения:
// каноническая запись каждого языка должна соответствовать формату колонки lang_code.
//
// Язык пользователя сохраняется не в том виде, в котором его прислал клиент,
// а в том, в котором его отдаёт локализатор приложения, поэтому ограничение
// колонки удерживается только здесь: язык со скриптовым сабтегом ("zh-Hans"),
// трёхбуквенным кодом ("fil") или регионом-числом ("es-419") в колонку
// не помещается и уронил бы запись уже в рантайме.
//
// Проверяется каноническая запись, а не исходная строка: mrlocale.NewBundle
// принимает и форму с подчёркиванием ("en_US"), которая после разбора
// становится "en-US" и хранению не мешает.
//
// Пустой список и дубликаты здесь намеренно не проверяются - это делает
// mrlocale.NewBundle, которому список передаётся следом. Неразбираемое имя,
// напротив, отвергается здесь, чтобы дать понятную ошибку до передачи в bundle.
//
// Это host-only reference-валидация уровня composition-root: предполагается, что её вызывает
// host-приложение из своего init-пути (внутри библиотеки она намеренно не вызывается). Конкретный
// проект может использовать её как есть либо написать собственную.
func ValidateLanguages(langs []string) error {
	for _, lang := range langs {
		tag, err := language.Parse(lang)
		if err != nil {
			return fmt.Errorf("error parsing language (name='%s'): %w", lang, err)
		}

		if code := tag.String(); !regexpStorableLang.MatchString(code) {
			return fmt.Errorf(
				"language '%s' is not storable as '%s': expected 2-letter code with optional 2-letter region (e.g. ru, ru-RU)",
				lang,
				code,
			)
		}
	}

	return nil
}
