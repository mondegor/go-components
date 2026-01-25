package settingtype

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
)

// Тип значения настройки.
const (
	String      Enum = iota + 1 // строковый тип настройки
	StringList                  // списочный тип строковых элементов настройки
	Integer                     // целочисленный тип настройки
	IntegerList                 // списочный тип целочисленных элементов настройки
	Boolean                     // логический тип настройки
)

const (
	enumLast = uint8(Boolean)
	enumName = "SettingType"
)

type (
	// Enum - тип значения настройки.
	Enum uint8
)

var (
	enumKeys = map[Enum]string{ //nolint:gochecknoglobals
		String:      "STRING",
		StringList:  "STRING_LIST",
		Integer:     "INTEGER",
		IntegerList: "INTEGER_LIST",
		Boolean:     "BOOLEAN",
	}

	enumValues = map[string]Enum{ //nolint:gochecknoglobals
		"STRING":       String,
		"STRING_LIST":  StringList,
		"INTEGER":      Integer,
		"INTEGER_LIST": IntegerList,
		"BOOLEAN":      Boolean,
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
	return enumKeys[e]
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
		return e.Set(uint8(val)) //nolint:gosec
	}

	return fmt.Errorf("invalid type assertion (type='%s', value='%+v')", enumName, value)
}

// Value implements the driver.Valuer interface.
func (e Enum) Value() (driver.Value, error) {
	return uint8(e), nil
}

// Parse - парсит указанное значение и если оно валидно, то устанавливает его числовое значение.
func Parse(value string) (Enum, error) {
	if parsedValue, ok := enumValues[value]; ok {
		return parsedValue, nil
	}

	return 0, fmt.Errorf("key is not found in source (source='%s', key='%s')", enumName, value)
}
