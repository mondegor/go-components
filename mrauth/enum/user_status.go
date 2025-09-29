package enum

import (
	"database/sql/driver"
	"encoding/json"
	"math"

	"github.com/mondegor/go-sysmess/mrerr/mr"
)

// Статусы пользователя.
const (
	UserStatusDraft    UserStatus = iota + 1 // черновик
	UserStatusEnabled                        // активный
	UserStatusDisabled                       // отключённый (админом)
	UserStatusBlocked                        // заблокированный (системой)
)

const (
	userStatusLast     = uint8(UserStatusBlocked)
	enumNameUserStatus = "UserStatus"
)

type (
	// UserStatus - статус элемента.
	UserStatus uint8
)

var (
	userStatusName = map[UserStatus]string{ //nolint:gochecknoglobals
		UserStatusDraft:    "DRAFT",
		UserStatusEnabled:  "ENABLED",
		UserStatusDisabled: "DISABLED",
		UserStatusBlocked:  "BLOCKED",
	}

	userStatusValue = map[string]UserStatus{ //nolint:gochecknoglobals
		"DRAFT":    UserStatusDraft,
		"ENABLED":  UserStatusEnabled,
		"DISABLED": UserStatusDisabled,
		"BLOCKED":  UserStatusBlocked,
	}
)

// ParseAndSet - парсит указанное значение и если оно валидно, то устанавливает его числовое значение.
func (e *UserStatus) ParseAndSet(value string) error {
	if parsedValue, ok := userStatusValue[value]; ok {
		*e = parsedValue

		return nil
	}

	return mr.ErrInternalKeyNotFoundInSource.New(value, enumNameUserStatus)
}

// Set - устанавливает указанное значение, если оно является enum значением.
func (e *UserStatus) Set(value uint8) error {
	if value > 0 && value <= userStatusLast {
		*e = UserStatus(value)

		return nil
	}

	return mr.ErrInternalKeyNotFoundInSource.New(value, enumNameUserStatus)
}

// String - возвращает значение в виде строки.
func (e UserStatus) String() string {
	return userStatusName[e]
}

// // Empty - сообщает, установлено ли enum значение.
// func (e UserStatus) Empty() bool {
// 	return e == 0
// }

// MarshalJSON - переводит enum значение в строковое представление.
func (e UserStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

// UnmarshalJSON - переводит строковое значение в enum представление.
func (e *UserStatus) UnmarshalJSON(data []byte) error {
	var value string

	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	return e.ParseAndSet(value)
}

// Scan implements the Scanner interface.
func (e *UserStatus) Scan(value any) error {
	if val, ok := value.(int64); ok && val >= 0 && val <= math.MaxUint8 {
		return e.Set(uint8(val)) //nolint:gosec
	}

	return mr.ErrInternalTypeAssertion.New(enumNameUserStatus, value)
}

// Value implements the driver.Valuer interface.
func (e UserStatus) Value() (driver.Value, error) {
	return uint8(e), nil
}
