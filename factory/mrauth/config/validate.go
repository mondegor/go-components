package config

import (
	"errors"
	"fmt"

	"github.com/mondegor/go-sysmess/mrlib/extstrings"
)

// ValidateRealms - comment func.
func ValidateRealms(realms []UserRealm, allRoles []string) error {
	uniqRealms := make(map[string]struct{}, len(realms))

	for _, realm := range realms {
		if _, ok := uniqRealms[realm.Name]; ok {
			return fmt.Errorf("duplicate realm name '%s'", realm.Name)
		}

		if realm.RegisterUserKind == "" {
			return fmt.Errorf("registerUser is empty for realm '%s'", realm.Name)
		}

		if realm.AuthToken.AccessType != "jwt" && realm.AuthToken.AccessType != "session" {
			return fmt.Errorf("invalid token type for realm (type='%s', realm='%s')", realm.AuthToken.AccessType, realm.Name)
		}

		uniqRealms[realm.Name] = struct{}{}

		if err := validateRealm(realm, allRoles); err != nil {
			return err
		}
	}

	return nil
}

func validateRealm(realm UserRealm, allRoles []string) error {
	uniqKinds := make(map[string]struct{}, len(realm.UserKinds))
	hasRegisterUser := realm.RegisterUserKind == "none"

	for _, kind := range realm.UserKinds {
		if _, ok := uniqKinds[kind.Name]; ok {
			return fmt.Errorf("duplicate user kind name for realm (kind='%s', realm='%s')", kind.Name, realm.Name)
		}

		uniqKinds[kind.Name] = struct{}{}

		if realm.RegisterUserKind == kind.Name {
			hasRegisterUser = true
		}

		for _, role := range kind.Roles {
			if !extstrings.InArray(role, allRoles) {
				return fmt.Errorf("role of user kind is not found in roles for realm (role='%s', kind='%s', realm='%s')", role, kind.Name, realm.Name)
			}
		}
	}

	if !hasRegisterUser {
		return fmt.Errorf("realm.RegisterUserKind is not found in realm.UserKinds for realm (kind='%s', realm='%s')", realm.RegisterUserKind, realm.Name)
	}

	return nil
}

// DefaultValuesRealm - comment func.
func DefaultValuesRealm(realms []UserRealm, dop OperationConfirm) []UserRealm {
	for i := range realms {
		rop := &realms[i].OperationConfirm

		if rop.TokenLength < 1 {
			rop.TokenLength = dop.TokenLength
		}

		if rop.CodeLength < 1 {
			rop.CodeLength = dop.CodeLength
		}

		if rop.SessionExpiry < 1 {
			rop.SessionExpiry = dop.SessionExpiry
		}

		rop.SendByEmail = defaultValuesCodeSender(rop.SendByEmail, dop.SendByEmail)
		rop.SendByPhone = defaultValuesCodeSender(rop.SendByPhone, dop.SendByPhone)
	}

	return realms
}

func defaultValuesCodeSender(cs, dcs CodeSender) CodeSender {
	if cs.MaxAttempts < 1 {
		cs.MaxAttempts = dcs.MaxAttempts
	}

	if cs.MaxResends < 1 {
		cs.MaxResends = dcs.MaxResends
	}

	if cs.MinResendTime < 1 {
		cs.MinResendTime = dcs.MinResendTime
	}

	return cs
}

// ValidateJWT - comment func.
func ValidateJWT(accessControl AccessControl) error {
	if !isJWTUsed(accessControl.Realms) {
		return nil
	}

	if accessControl.JWTMethod == "" {
		return errors.New("JWT method is required")
	}

	switch accessControl.JWTMethod {
	case "HS256", "HS512": // TODO: "ES256", "ES512"
	default:
		return errors.New("invalid JWT method")
	}

	if accessControl.JWTSecret == "" {
		return errors.New("JWT secret is required")
	}

	return nil
}

func isJWTUsed(realms []UserRealm) bool {
	for _, realm := range realms {
		if realm.AuthToken.AccessType == "jwt" {
			return true
		}
	}

	return false
}
