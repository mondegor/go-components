package enum

import (
	"database/sql/driver"
	"encoding/json"

	"github.com/mondegor/go-webcore/mrcore"
)

const (
	_                    ItemStatus = iota
	ItemStatusReady                 // ItemStatusReady - элемент очереди готов для обработки
	ItemStatusProcessing            // ItemStatusProcessing - элемент очереди находится в обработке
	ItemStatusRetry                 // ItemStatusRetry - элемент очереди завершился с ошибкой и ожидает повторной обработки

	itemStatusLast     = uint8(ItemStatusRetry)
	enumNameItemStatus = "ItemStatus"
)

type (
	// ItemStatus - статус элемента в очереди.
	ItemStatus uint8
)

var (
	itemStatusName = map[ItemStatus]string{ //nolint:gochecknoglobals
		ItemStatusReady:      "READY",
		ItemStatusProcessing: "PROCESSING",
		ItemStatusRetry:      "RETRY",
	}

	itemStatusValue = map[string]ItemStatus{ //nolint:gochecknoglobals
		"READY":      ItemStatusReady,
		"PROCESSING": ItemStatusProcessing,
		"RETRY":      ItemStatusRetry,
	}
)

// ParseAndSet - парсит указанное значение и если оно валидно, то устанавливает его числовое значение.
func (e *ItemStatus) ParseAndSet(value string) error {
	if parsedValue, ok := itemStatusValue[value]; ok {
		*e = parsedValue

		return nil
	}

	return mrcore.ErrInternalKeyNotFoundInSource.New(value, enumNameItemStatus)
}

// Set - устанавливает указанное значение, если оно является enum значением.
func (e *ItemStatus) Set(value uint8) error {
	if value > 0 && value <= itemStatusLast {
		*e = ItemStatus(value)

		return nil
	}

	return mrcore.ErrInternalKeyNotFoundInSource.New(value, enumNameItemStatus)
}

// String - возвращается значение в виде строки.
func (e ItemStatus) String() string {
	return itemStatusName[e]
}

// Empty - проверяет, что enum значение не установлено.
func (e ItemStatus) Empty() bool {
	return e == 0
}

// MarshalJSON - переводит enum значение в строковое представление.
func (e ItemStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

// UnmarshalJSON - переводит строковое значение в enum представление.
func (e *ItemStatus) UnmarshalJSON(data []byte) error {
	var value string

	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	return e.ParseAndSet(value)
}

// Scan implements the Scanner interface.
func (e *ItemStatus) Scan(value any) error {
	if val, ok := value.(int64); ok {
		return e.Set(uint8(val))
	}

	return mrcore.ErrInternalTypeAssertion.New(enumNameItemStatus, value)
}

// Value implements the driver.Valuer interface.
func (e ItemStatus) Value() (driver.Value, error) {
	return uint8(e), nil
}
