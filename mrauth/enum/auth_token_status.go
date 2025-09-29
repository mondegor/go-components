package enum

import (
	"database/sql/driver"
	"encoding/json"
	"math"

	"github.com/mondegor/go-sysmess/mrerr/mr"
)

// Статусы токена авторизации.
const (
	AuthTokenStatusOpened            AuthTokenStatus = iota + 1 // действующий
	AuthTokenStatusClosed                                       // закрытый
	AuthTokenStatusRevoked                                      // отозванный
	AuthTokenStatusUnexpectedRevoked                            // неожиданно отозванный
)

const (
	authTokenStatusLast     = uint8(AuthTokenStatusUnexpectedRevoked)
	enumNameAuthTokenStatus = "AuthTokenStatus"
)

type (
	// AuthTokenStatus - статус элемента.
	AuthTokenStatus uint8
)

var (
	authTokenStatusName = map[AuthTokenStatus]string{ //nolint:gochecknoglobals
		AuthTokenStatusOpened:            "OPENED",
		AuthTokenStatusClosed:            "CLOSED",
		AuthTokenStatusRevoked:           "REVOKED",
		AuthTokenStatusUnexpectedRevoked: "UNEXPECTED_REVOKED",
	}

	authTokenStatusValue = map[string]AuthTokenStatus{ //nolint:gochecknoglobals
		"OPENED":             AuthTokenStatusOpened,
		"CLOSED":             AuthTokenStatusClosed,
		"REVOKED":            AuthTokenStatusRevoked,
		"UNEXPECTED_REVOKED": AuthTokenStatusUnexpectedRevoked,
	}
)

// ParseAndSet - парсит указанное значение и если оно валидно, то устанавливает его числовое значение.
func (e *AuthTokenStatus) ParseAndSet(value string) error {
	if parsedValue, ok := authTokenStatusValue[value]; ok {
		*e = parsedValue

		return nil
	}

	return mr.ErrInternalKeyNotFoundInSource.New(value, enumNameAuthTokenStatus)
}

// Set - устанавливает указанное значение, если оно является enum значением.
func (e *AuthTokenStatus) Set(value uint8) error {
	if value > 0 && value <= authTokenStatusLast {
		*e = AuthTokenStatus(value)

		return nil
	}

	return mr.ErrInternalKeyNotFoundInSource.New(value, enumNameAuthTokenStatus)
}

// String - возвращает значение в виде строки.
func (e AuthTokenStatus) String() string {
	return authTokenStatusName[e]
}

// // Empty - сообщает, установлено ли enum значение.
// func (e AuthTokenStatus) Empty() bool {
// 	return e == 0
// }

// MarshalJSON - переводит enum значение в строковое представление.
func (e AuthTokenStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

// UnmarshalJSON - переводит строковое значение в enum представление.
func (e *AuthTokenStatus) UnmarshalJSON(data []byte) error {
	var value string

	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	return e.ParseAndSet(value)
}

// Scan implements the Scanner interface.
func (e *AuthTokenStatus) Scan(value any) error {
	if val, ok := value.(int64); ok && val >= 0 && val <= math.MaxUint8 {
		return e.Set(uint8(val)) //nolint:gosec
	}

	return mr.ErrInternalTypeAssertion.New(enumNameAuthTokenStatus, value)
}

// Value implements the driver.Valuer interface.
func (e AuthTokenStatus) Value() (driver.Value, error) {
	return uint8(e), nil
}
