package operationtype

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
)

// Типы защищённых операций. Клиенту из них отдаются только типы операций личного кабинета
// (см. unit.PendingOperationTypes), остальные используются в хранилище и журнале.
const (
	AuthorizeUser      Enum = iota + 1 // вход в аккаунт
	CreateUser                         // регистрация пользователя
	ChangeEmail                        // смена емаила, шаг 1: подтверждение владения аккаунтом
	ChangeEmailConfirm                 // смена емаила, шаг 2: подтверждение владения новым адресом
	ChangePhone                        // смена телефона
	ChangePassword                     // смена пароля
	ChangeTOTP                         // привязка TOTP
	Disable2FA                         // отключение 2FA
	RegenerateRecovery                 // перегенерация аварийных кодов
)

const (
	enumLast = uint8(RegenerateRecovery)
	enumName = "OperationType"
)

type (
	// Enum - тип защищённой операции.
	Enum uint8
)

//nolint:gochecknoglobals
var (
	enumKeys = map[Enum]string{
		AuthorizeUser:      "AUTHORIZE_USER",
		CreateUser:         "CREATE_USER",
		ChangeEmail:        "CHANGE_EMAIL",
		ChangeEmailConfirm: "CHANGE_EMAIL_CONFIRM",
		ChangePhone:        "CHANGE_PHONE",
		ChangePassword:     "CHANGE_PASSWORD",
		ChangeTOTP:         "CHANGE_TOTP",
		Disable2FA:         "DISABLE_2FA",
		RegenerateRecovery: "REGENERATE_RECOVERY",
	}

	enumValues = map[string]Enum{
		"AUTHORIZE_USER":       AuthorizeUser,
		"CREATE_USER":          CreateUser,
		"CHANGE_EMAIL":         ChangeEmail,
		"CHANGE_EMAIL_CONFIRM": ChangeEmailConfirm,
		"CHANGE_PHONE":         ChangePhone,
		"CHANGE_PASSWORD":      ChangePassword,
		"CHANGE_TOTP":          ChangeTOTP,
		"DISABLE_2FA":          Disable2FA,
		"REGENERATE_RECOVERY":  RegenerateRecovery,
	}
)

// Set - устанавливает указанное значение, если оно является enum значением.
func (e *Enum) Set(value uint8) error {
	if value > 0 && value <= enumLast {
		*e = Enum(value)

		return nil
	}

	return fmt.Errorf("value '%d' is not found in enum set '%s'", value, enumName)
}

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

// UnmarshalJSON - переводит строковое значение в enum представление.
func (e *Enum) UnmarshalJSON(data []byte) error {
	var value string

	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("unmarshal error (source='%s'): %w", enumName, err)
	}

	val, err := Parse(value)
	if err != nil {
		return err
	}

	*e = val

	return nil
}

// Scan implements the Scanner interface.
func (e *Enum) Scan(value any) error {
	if val, ok := value.(int64); ok && val >= 0 && val <= math.MaxUint8 {
		return e.Set(uint8(val))
	}

	return fmt.Errorf("invalid type assertion (type='%s', value='%+v')", enumName, value)
}

// Value implements the driver.Valuer interface.
func (e Enum) Value() (driver.Value, error) {
	return uint8(e), nil
}

// Parse - парсит указанное значение и если оно валидно, то возвращает его числовое значение.
func Parse(value string) (Enum, error) {
	if parsedValue, ok := enumValues[value]; ok {
		return parsedValue, nil
	}

	return 0, fmt.Errorf("key is not found in source (source='%s', key='%s')", enumName, value)
}
