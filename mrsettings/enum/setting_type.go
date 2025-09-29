package enum

import (
	"database/sql/driver"
	"encoding/json"
	"math"

	"github.com/mondegor/go-sysmess/mrerr/mr"
)

// Тип значения настройки.
const (
	SettingTypeString      SettingType = iota + 1 // строковый тип настройки
	SettingTypeStringList                         // списочный тип строковых элементов настройки
	SettingTypeInteger                            // целочисленный тип настройки
	SettingTypeIntegerList                        // списочный тип целочисленных элементов настройки
	SettingTypeBoolean                            // логический тип настройки
)

const (
	settingTypeLast     = uint8(SettingTypeBoolean)
	enumNameSettingType = "SettingType"
)

type (
	// SettingType - тип значения настройки.
	SettingType uint8
)

var (
	settingTypeName = map[SettingType]string{ //nolint:gochecknoglobals
		SettingTypeString:      "STRING",
		SettingTypeStringList:  "STRING_LIST",
		SettingTypeInteger:     "INTEGER",
		SettingTypeIntegerList: "INTEGER_LIST",
		SettingTypeBoolean:     "BOOLEAN",
	}

	settingTypeValue = map[string]SettingType{ //nolint:gochecknoglobals
		"STRING":       SettingTypeString,
		"STRING_LIST":  SettingTypeStringList,
		"INTEGER":      SettingTypeInteger,
		"INTEGER_LIST": SettingTypeIntegerList,
		"BOOLEAN":      SettingTypeBoolean,
	}
)

// ParseAndSet - парсит указанное значение и если оно валидно, то устанавливает его числовое значение.
func (e *SettingType) ParseAndSet(value string) error {
	if parsedValue, ok := settingTypeValue[value]; ok {
		*e = parsedValue

		return nil
	}

	return mr.ErrInternalKeyNotFoundInSource.New(value, enumNameSettingType)
}

// Set - устанавливает указанное значение, если оно является enum значением.
func (e *SettingType) Set(value uint8) error {
	if value > 0 && value <= settingTypeLast {
		*e = SettingType(value)

		return nil
	}

	return mr.ErrInternalKeyNotFoundInSource.New(value, enumNameSettingType)
}

// String - возвращается значение в виде строки.
func (e SettingType) String() string {
	return settingTypeName[e]
}

// // Empty - сообщает, установлено ли enum значение.
// func (e SettingType) Empty() bool {
// 	return e == 0
// }

// MarshalJSON - переводит enum значение в строковое представление.
func (e SettingType) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

// UnmarshalJSON - переводит строковое значение в enum представление.
func (e *SettingType) UnmarshalJSON(data []byte) error {
	var value string

	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	return e.ParseAndSet(value)
}

// Scan implements the Scanner interface.
func (e *SettingType) Scan(value any) error {
	if val, ok := value.(int64); ok && val >= 0 && val <= math.MaxUint8 {
		return e.Set(uint8(val)) //nolint:gosec
	}

	return mr.ErrInternalTypeAssertion.New(enumNameSettingType, value)
}

// Value implements the driver.Valuer interface.
func (e SettingType) Value() (driver.Value, error) {
	return uint8(e), nil
}
