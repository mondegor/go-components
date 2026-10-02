package authtokentype

import (
	"database/sql/driver"
)

// Типы токена авторизации.
const (
	Access  Enum = iota + 1 // токен доступа
	Refresh                 // токен обновления
	API                     // токен программного доступа
)

type (
	// Enum - тип токена.
	Enum uint8
)

//nolint:gochecknoglobals
var (
	enumKeys = map[Enum]string{
		Access:  "ACCESS",
		Refresh: "REFRESH",
		API:     "API",
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
