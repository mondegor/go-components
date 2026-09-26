package operationtype

import (
	"encoding/json"
	"fmt"
)

// Типы защищённых операций личного кабинета, отдаваемые клиенту
// (операции входа и регистрации сюда не входят).
const (
	ChangeEmail        Enum = iota + 1 // смена емаила, шаг 1: подтверждение владения аккаунтом
	ChangeEmailConfirm                 // смена емаила, шаг 2: подтверждение владения новым адресом
	ChangePhone                        // смена телефона
	ChangePassword                     // установка пароля (2FA)
	ChangeTOTP                         // установка TOTP генератора (2FA)
	RegenerateRecovery                 // перевыпуск одноразовых аварийных кодов
	Disable2FA                         // отключение 2FA
)

const (
	enumName = "OperationType"
)

type (
	// Enum - тип защищённой операции.
	Enum uint8
)

//nolint:gochecknoglobals
var (
	enumKeys = map[Enum]string{
		ChangeEmail:        "CHANGE_EMAIL",
		ChangeEmailConfirm: "CHANGE_EMAIL_CONFIRM",
		ChangePhone:        "CHANGE_PHONE",
		ChangePassword:     "CHANGE_PASSWORD",
		ChangeTOTP:         "CHANGE_TOTP",
		RegenerateRecovery: "REGENERATE_RECOVERY",
		Disable2FA:         "DISABLE_2FA",
	}

	enumValues = map[string]Enum{
		"CHANGE_EMAIL":         ChangeEmail,
		"CHANGE_EMAIL_CONFIRM": ChangeEmailConfirm,
		"CHANGE_PHONE":         ChangePhone,
		"CHANGE_PASSWORD":      ChangePassword,
		"CHANGE_TOTP":          ChangeTOTP,
		"REGENERATE_RECOVERY":  RegenerateRecovery,
		"DISABLE_2FA":          Disable2FA,
	}
)

// String - возвращает значение в виде строки.
func (e Enum) String() string {
	if v, ok := enumKeys[e]; ok {
		return v
	}

	return "UNKNOWN"
}

// MarshalJSON - переводит enum значение в строковое представление.
func (e Enum) MarshalJSON() ([]byte, error) {
	bytes, err := json.Marshal(e.String())
	if err != nil {
		return nil, fmt.Errorf("marshal error (source='%s'): %w", enumName, err)
	}

	return bytes, nil
}

// Parse - парсит указанное значение и если оно валидно, то возвращает его числовое значение.
func Parse(value string) (Enum, error) {
	if parsedValue, ok := enumValues[value]; ok {
		return parsedValue, nil
	}

	return 0, fmt.Errorf("key is not found in source (source='%s', key='%s')", enumName, value)
}
