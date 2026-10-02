package settingtype

import (
	"database/sql/driver"
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

//nolint:gochecknoglobals
var (
	enumKeys = map[Enum]string{
		String:      "STRING",
		StringList:  "STRING_LIST",
		Integer:     "INTEGER",
		IntegerList: "INTEGER_LIST",
		Boolean:     "BOOLEAN",
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
