package enum

import (
	"database/sql/driver"
	"encoding/json"

	"github.com/mondegor/go-webcore/mrcore"
)

const (
	_                      SettingType = iota
	SettingTypeString                  // SettingTypeString - строковый тип настройки
	SettingTypeStringList              // SettingTypeStringList - списочный тип строковых элементов настройки
	SettingTypeInteger                 // SettingTypeInteger - целочисленный тип настройки
	SettingTypeIntegerList             // SettingTypeIntegerList - списочный тип целочисленных элементов настройки
	SettingTypeBoolean                 // SettingTypeBoolean - логический тип настройки

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

	return mrcore.ErrInternalKeyNotFoundInSource.New(value, enumNameSettingType)
}

// Set - устанавливает указанное значение, если оно является enum значением.
func (e *SettingType) Set(value uint8) error {
	if value > 0 && value <= settingTypeLast {
		*e = SettingType(value)

		return nil
	}

	return mrcore.ErrInternalKeyNotFoundInSource.New(value, enumNameSettingType)
}

// String - возвращается значение в виде строки.
func (e SettingType) String() string {
	return settingTypeName[e]
}

// Empty - проверяет, что enum значение не установлено.
func (e SettingType) Empty() bool {
	return e == 0
}

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
	if val, ok := value.(int64); ok {
		return e.Set(uint8(val))
	}

	return mrcore.ErrInternalTypeAssertion.New(enumNameSettingType, value)
}

// Value implements the driver.Valuer interface.
func (e SettingType) Value() (driver.Value, error) {
	return uint8(e), nil
}

// ParseSettingTypeList - парсит массив строковых значений и
// возвращает соответствующий массив enum значений.
func ParseSettingTypeList(items []string) ([]SettingType, error) {
	var tmp SettingType

	parsedItems := make([]SettingType, len(items))

	for i := range items {
		if err := tmp.ParseAndSet(items[i]); err != nil {
			return nil, err
		}

		parsedItems[i] = tmp
	}

	return parsedItems, nil
}
