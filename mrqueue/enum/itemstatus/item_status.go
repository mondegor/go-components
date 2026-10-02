package itemstatus

import (
	"database/sql/driver"
)

// Статусы элементов очереди.
const (
	Ready      Enum = iota + 1 // элемент очереди готов для обработки
	Processing                 // элемент очереди находится в обработке
	Retry                      // элемент очереди завершился с ошибкой и ожидает повторной обработки
)

type (
	// Enum - статус элемента в очереди.
	Enum uint8
)

//nolint:gochecknoglobals
var (
	enumKeys = map[Enum]string{
		Ready:      "READY",
		Processing: "PROCESSING",
		Retry:      "RETRY",
	}
)

// String - возвращает значение в виде строки.
func (e Enum) String() string {
	if v, ok := enumKeys[e]; ok {
		return v
	}

	return "UNKNOWN"
}

// Value implements the driver.Valuer interface.
func (e Enum) Value() (driver.Value, error) {
	return uint8(e), nil
}
