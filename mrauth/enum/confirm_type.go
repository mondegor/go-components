package enum

import (
	"database/sql/driver"
	"encoding/json"
	"math"

	"github.com/mondegor/go-sysmess/mrerr/mr"
)

// Методы подтверждения подлинности пользователя.
const (
	ConfirmMethodEmail    ConfirmMethod = iota + 1 // по емаилу
	ConfirmMethodPhone                             // по телефону
	ConfirmMethodPassword                          // по паролю
	ConfirmMethodTOTP                              // по TOTP
)

const (
	confirmMethodLast     = uint8(ConfirmMethodTOTP)
	enumNameConfirmMethod = "ConfirmMethod"
)

type (
	// ConfirmMethod - статус элемента.
	ConfirmMethod uint8
)

var (
	confirmTypeName = map[ConfirmMethod]string{ //nolint:gochecknoglobals
		ConfirmMethodEmail:    "EMAIL",
		ConfirmMethodPhone:    "PHONE",
		ConfirmMethodPassword: "PASSWORD",
		ConfirmMethodTOTP:     "TOTP",
	}

	confirmTypeValue = map[string]ConfirmMethod{ //nolint:gochecknoglobals
		"EMAIL":    ConfirmMethodEmail,
		"PHONE":    ConfirmMethodPhone,
		"PASSWORD": ConfirmMethodPassword,
		"TOTP":     ConfirmMethodTOTP,
	}
)

// ParseAndSet - парсит указанное значение и если оно валидно, то устанавливает его числовое значение.
func (e *ConfirmMethod) ParseAndSet(value string) error {
	if parsedValue, ok := confirmTypeValue[value]; ok {
		*e = parsedValue

		return nil
	}

	return mr.ErrInternalKeyNotFoundInSource.New(value, enumNameConfirmMethod)
}

// Set - устанавливает указанное значение, если оно является enum значением.
func (e *ConfirmMethod) Set(value uint8) error {
	if value > 0 && value <= confirmMethodLast {
		*e = ConfirmMethod(value)

		return nil
	}

	return mr.ErrInternalKeyNotFoundInSource.New(value, enumNameConfirmMethod)
}

// String - возвращает значение в виде строки.
func (e ConfirmMethod) String() string {
	return confirmTypeName[e]
}

// // Empty - сообщает, установлено ли enum значение.
// func (e ConfirmMethod) Empty() bool {
// 	return e == 0
// }

// MarshalJSON - переводит enum значение в строковое представление.
func (e ConfirmMethod) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

// UnmarshalJSON - переводит строковое значение в enum представление.
func (e *ConfirmMethod) UnmarshalJSON(data []byte) error {
	var value string

	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	return e.ParseAndSet(value)
}

// Scan implements the Scanner interface.
func (e *ConfirmMethod) Scan(value any) error {
	if val, ok := value.(int64); ok && val >= 0 && val <= math.MaxUint8 {
		return e.Set(uint8(val)) //nolint:gosec
	}

	return mr.ErrInternalTypeAssertion.New(enumNameConfirmMethod, value)
}

// Value implements the driver.Valuer interface.
func (e ConfirmMethod) Value() (driver.Value, error) {
	return uint8(e), nil
}
