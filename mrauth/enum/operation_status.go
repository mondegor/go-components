package enum

import (
	"database/sql/driver"
	"encoding/json"
	"math"

	"github.com/mondegor/go-sysmess/mrerr/mr"
)

// Статусы операции.
const (
	OperationStatusOpened    OperationStatus = iota + 1 // открыт
	OperationStatusConfirmed                            // подтверждена
	// OperationStatusUpdating                             // на обновлении.
)

const (
	operationStatusLast     = uint8(OperationStatusConfirmed)
	enumNameOperationStatus = "OperationStatus"
)

type (
	// OperationStatus - статус элемента.
	OperationStatus uint8
)

var (
	operationStatusName = map[OperationStatus]string{ //nolint:gochecknoglobals
		OperationStatusOpened:    "OPENED",
		OperationStatusConfirmed: "CONFIRMED",
		// OperationStatusUpdating:  "UPDATING",
	}

	operationStatusValue = map[string]OperationStatus{ //nolint:gochecknoglobals
		"OPENED":    OperationStatusOpened,
		"CONFIRMED": OperationStatusConfirmed,
		// "UPDATING":  OperationStatusUpdating,
	}
)

// ParseAndSet - парсит указанное значение и если оно валидно, то устанавливает его числовое значение.
func (e *OperationStatus) ParseAndSet(value string) error {
	if parsedValue, ok := operationStatusValue[value]; ok {
		*e = parsedValue

		return nil
	}

	return mr.ErrInternalKeyNotFoundInSource.New(value, enumNameOperationStatus)
}

// Set - устанавливает указанное значение, если оно является enum значением.
func (e *OperationStatus) Set(value uint8) error {
	if value > 0 && value <= operationStatusLast {
		*e = OperationStatus(value)

		return nil
	}

	return mr.ErrInternalKeyNotFoundInSource.New(value, enumNameOperationStatus)
}

// String - возвращает значение в виде строки.
func (e OperationStatus) String() string {
	return operationStatusName[e]
}

// // Empty - сообщает, установлено ли enum значение.
// func (e OperationStatus) Empty() bool {
// 	return e == 0
// }

// MarshalJSON - переводит enum значение в строковое представление.
func (e OperationStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

// UnmarshalJSON - переводит строковое значение в enum представление.
func (e *OperationStatus) UnmarshalJSON(data []byte) error {
	var value string

	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	return e.ParseAndSet(value)
}

// Scan implements the Scanner interface.
func (e *OperationStatus) Scan(value any) error {
	if val, ok := value.(int64); ok && val >= 0 && val <= math.MaxUint8 {
		return e.Set(uint8(val)) //nolint:gosec
	}

	return mr.ErrInternalTypeAssertion.New(enumNameOperationStatus, value)
}

// Value implements the driver.Valuer interface.
func (e OperationStatus) Value() (driver.Value, error) {
	return uint8(e), nil
}
