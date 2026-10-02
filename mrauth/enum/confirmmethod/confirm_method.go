package confirmmethod

import (
	"encoding/json"
	"fmt"
)

// Методы подтверждения подлинности пользователя.
const (
	Unspecified Enum = iota // метод не указан (pre-op события)
	Email                   // по емаилу
	Phone                   // по телефону
	Password                // по паролю
	TOTP                    // по TOTP
	Recovery                // по аварийному коду (только последним звеном цепочки)
)

const (
	enumName = "ConfirmMethod"
)

type (
	// Enum - метод подтверждения подлинности пользователя.
	Enum uint8
)

//nolint:gochecknoglobals
var (
	enumKeys = map[Enum]string{
		Unspecified: "UNSPECIFIED",
		Email:       "EMAIL",
		Phone:       "PHONE",
		Password:    "PASSWORD",
		TOTP:        "TOTP",
		Recovery:    "RECOVERY",
	}

	enumValues = map[string]Enum{
		"UNSPECIFIED": Unspecified,
		"EMAIL":       Email,
		"PHONE":       Phone,
		"PASSWORD":    Password,
		"TOTP":        TOTP,
		"RECOVERY":    Recovery,
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

// Parse - парсит указанное значение и если оно валидно, то возвращает его числовое значение.
func Parse(value string) (Enum, error) {
	if parsedValue, ok := enumValues[value]; ok {
		return parsedValue, nil
	}

	return 0, fmt.Errorf("key is not found in source (source='%s', key='%s')", enumName, value)
}
