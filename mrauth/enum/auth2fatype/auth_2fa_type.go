package auth2fatype

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
)

// Типы двухфакторной авторизации.
const (
	None     Enum = iota // отсутствует (отключена)
	Password             // по паролю
	TOTP                 // по TOTP
)

const (
	enumLast = uint8(TOTP)
	enumName = "Auth2FAType"
)

type (
	// Enum - тип двухфакторной авторизации.
	Enum uint8
)

//nolint:gochecknoglobals
var (
	enumKeys = map[Enum]string{
		None:     "NONE",
		Password: "PASSWORD",
		TOTP:     "TOTP",
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
