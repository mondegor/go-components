package enum

import (
	"database/sql/driver"
	"encoding/json"
	"math"

	"github.com/mondegor/go-sysmess/mrerr/mr"
)

// Типы двухфакторной авторизации.
const (
	Auth2faTypeNone     Auth2faType = iota // отключена
	Auth2faTypePassword                    // по паролю
	Auth2faTypeTOTP                        // по TOTP
)

const (
	auth2faTypeLast     = uint8(Auth2faTypeTOTP)
	enumNameAuth2faType = "Auth2faType"
)

type (
	// Auth2faType - статус элемента.
	Auth2faType uint8
)

var (
	auth2faTypeName = map[Auth2faType]string{ //nolint:gochecknoglobals
		Auth2faTypeNone:     "NONE",
		Auth2faTypePassword: "PASSWORD",
		Auth2faTypeTOTP:     "TOTP",
	}

	auth2faTypeValue = map[string]Auth2faType{ //nolint:gochecknoglobals
		"NONE":     Auth2faTypeNone,
		"PASSWORD": Auth2faTypePassword,
		"TOTP":     Auth2faTypeTOTP,
	}
)

// ParseAndSet - парсит указанное значение и если оно валидно, то устанавливает его числовое значение.
func (e *Auth2faType) ParseAndSet(value string) error {
	if parsedValue, ok := auth2faTypeValue[value]; ok {
		*e = parsedValue

		return nil
	}

	return mr.ErrInternalKeyNotFoundInSource.New(value, enumNameAuth2faType)
}

// Set - устанавливает указанное значение, если оно является enum значением.
func (e *Auth2faType) Set(value uint8) error {
	if value > 0 && value <= auth2faTypeLast {
		*e = Auth2faType(value)

		return nil
	}

	return mr.ErrInternalKeyNotFoundInSource.New(value, enumNameAuth2faType)
}

// String - возвращает значение в виде строки.
func (e Auth2faType) String() string {
	return auth2faTypeName[e]
}

// // Empty - сообщает, установлено ли enum значение.
// func (e Auth2faType) Empty() bool {
// 	return e == 0
// }

// MarshalJSON - переводит enum значение в строковое представление.
func (e Auth2faType) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

// UnmarshalJSON - переводит строковое значение в enum представление.
func (e *Auth2faType) UnmarshalJSON(data []byte) error {
	var value string

	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	return e.ParseAndSet(value)
}

// Scan implements the Scanner interface.
func (e *Auth2faType) Scan(value any) error {
	if val, ok := value.(int64); ok && val >= 0 && val <= math.MaxUint8 {
		return e.Set(uint8(val)) //nolint:gosec
	}

	return mr.ErrInternalTypeAssertion.New(enumNameAuth2faType, value)
}

// Value implements the driver.Valuer interface.
func (e Auth2faType) Value() (driver.Value, error) {
	return uint8(e), nil
}
