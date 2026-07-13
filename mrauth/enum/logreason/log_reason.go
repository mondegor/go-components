package logreason

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
)

// Причины провала/блокировки записи журнала защищённых операций.
// Нулевое значение UNSPECIFIED используется при успешных исходах.
const (
	Unspecified       Enum = iota // причина не указана (успех)
	WrongCode                     // неверный код подтверждения
	AttemptsExhausted             // исчерпаны попытки подтверждения
	Throttled                     // сработал троттлинг/анти-спам
	TokenReuse                    // повторное использование refresh-токена
	AccessForbidden               // обращение к чужой операции
	TOTPReplay                    // гонка 2FA / повтор TOTP-шага
	Expired                       // операция истекла (зарезервировано: сейчас не выставляется)
	NotConfirmed                  // операция не подтверждена
	LoginNotExists                // логин не существует
	SessionLimit                  // превышен лимит сессий
)

const (
	enumLast = uint8(SessionLimit)
	enumName = "LogReason"
)

type (
	// Enum - причина провала/блокировки записи журнала.
	Enum uint8
)

//nolint:gochecknoglobals
var (
	enumKeys = map[Enum]string{
		Unspecified:       "UNSPECIFIED",
		WrongCode:         "WRONG_CODE",
		AttemptsExhausted: "ATTEMPTS_EXHAUSTED",
		Throttled:         "THROTTLED",
		TokenReuse:        "TOKEN_REUSE",
		AccessForbidden:   "ACCESS_FORBIDDEN",
		TOTPReplay:        "TOTP_REPLAY",
		Expired:           "EXPIRED",
		NotConfirmed:      "NOT_CONFIRMED",
		LoginNotExists:    "LOGIN_NOT_EXISTS",
		SessionLimit:      "SESSION_LIMIT",
	}

	enumValues = map[string]Enum{
		"UNSPECIFIED":        Unspecified,
		"WRONG_CODE":         WrongCode,
		"ATTEMPTS_EXHAUSTED": AttemptsExhausted,
		"THROTTLED":          Throttled,
		"TOKEN_REUSE":        TokenReuse,
		"ACCESS_FORBIDDEN":   AccessForbidden,
		"TOTP_REPLAY":        TOTPReplay,
		"EXPIRED":            Expired,
		"NOT_CONFIRMED":      NotConfirmed,
		"LOGIN_NOT_EXISTS":   LoginNotExists,
		"SESSION_LIMIT":      SessionLimit,
	}
)

// Set - устанавливает указанное значение, если оно является enum значением.
func (e *Enum) Set(value uint8) error {
	if value <= enumLast {
		*e = Enum(value)

		return nil
	}

	return fmt.Errorf("value '%d' is not found in enum set '%s'", value, enumName)
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

// UnmarshalJSON - переводит строковое значение в enum представление.
func (e *Enum) UnmarshalJSON(data []byte) error {
	var value string

	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("unmarshal error (source='%s'): %w", enumName, err)
	}

	val, err := Parse(value)
	if err != nil {
		return err
	}

	*e = val

	return nil
}

// Scan implements the Scanner interface.
func (e *Enum) Scan(value any) error {
	if val, ok := value.(int64); ok && val >= 0 && val <= math.MaxUint8 {
		return e.Set(uint8(val))
	}

	return fmt.Errorf("invalid type assertion (type='%s', value='%+v')", enumName, value)
}

// Value implements the driver.Valuer interface.
func (e Enum) Value() (driver.Value, error) {
	return uint8(e), nil
}

// Parse - парсит указанное значение и если оно валидно, то устанавливает его числовое значение.
func Parse(value string) (Enum, error) {
	if parsedValue, ok := enumValues[value]; ok {
		return parsedValue, nil
	}

	return 0, fmt.Errorf("key is not found in source (source='%s', key='%s')", enumName, value)
}
