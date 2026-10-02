package passwordacceptstatus

import (
	"encoding/json"
	"fmt"
)

// Статусы приёма пароля как пароля 2FA.
const (
	Accepted           Enum = iota + 1 // пароль будет принят
	TooWeak                            // надёжность пароля ниже порога
	RecoveryCodeFormat                 // пароль имеет формат аварийного кода
)

const (
	enumName = "PasswordAcceptStatus"
)

type (
	// Enum - статус приёма пароля как пароля 2FA: принят либо причина отказа.
	Enum uint8
)

//nolint:gochecknoglobals
var enumKeys = map[Enum]string{
	Accepted:           "ACCEPTED",
	TooWeak:            "TOO_WEAK",
	RecoveryCodeFormat: "RECOVERY_CODE_FORMAT",
}

// String - возвращает значение в виде строки.
func (e Enum) String() string {
	if v, ok := enumKeys[e]; ok {
		return v
	}

	return "UNKNOWN"
}

// MarshalJSON - переводит enum значение в строковое представление.
func (e Enum) MarshalJSON() ([]byte, error) {
	bytes, err := json.Marshal(e.String())
	if err != nil {
		return nil, fmt.Errorf("marshal error (source='%s'): %w", enumName, err)
	}

	return bytes, nil
}
