package config

import (
	"fmt"
	"slices"
)

const (
	accessTypeJWT     = "jwt"
	accessTypeSession = "session"

	// minHMACSecretHS256 - минимальная длина HMAC-секрета для HS256 (256 бит, RFC 7518 §3.2).
	minHMACSecretHS256 = 32

	// minHMACSecretHS512 - минимальная длина HMAC-секрета для HS512 (512 бит, RFC 7518 §3.2).
	minHMACSecretHS512 = 64
)

// ValidateRealms - проверяет конфигурацию realm'ов: уникальность имён, корректность типов токенов,
// TTL jwt-токенов и принадлежность ролей известному набору.
func ValidateRealms(realms []UserRealm, allRoles []string) error {
	uniqRealms := make(map[string]bool, len(realms))

	for _, realm := range realms {
		if uniqRealms[realm.Name] {
			return fmt.Errorf("duplicate realm name '%s'", realm.Name)
		}

		if realm.RegisterUserKind == "" {
			return fmt.Errorf("registerUser is empty for realm '%s'", realm.Name)
		}

		if realm.AuthToken.AccessType != accessTypeJWT && realm.AuthToken.AccessType != accessTypeSession {
			return fmt.Errorf("invalid token type for realm (type='%s', realm='%s')", realm.AuthToken.AccessType, realm.Name)
		}

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
