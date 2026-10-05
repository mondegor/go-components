package securityevent

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
)

// Типы событий журнала безопасности пользователя: только состоявшиеся значимые события,
// неудачные попытки по отдельности сюда не попадают.
const (
	SignedIn                 Enum = iota + 1 // вход в аккаунт (в том числе завершающий регистрацию)
	Auth2FAEnabled                           // включение 2FA
	Auth2FADisabled                          // снятие 2FA
	RecoveryCodeUsed                         // трата аварийного кода
	RecoveryCodesRegenerated                 // перевыпуск аварийных кодов
	EmailChangeRequested                     // запрошена смена емаила (создан токен смены адреса)
	EmailChanged                             // смена емаила завершена
	EmailChangeRevoked                       // токен смены емаила отозван пользователем
	PhoneChanged                             // смена телефона
	SessionsClosed                           // закрытие сессий из списка
	TokenReuseDetected                       // повторное использование refresh-токена открытой сессии
)

const (
	enumLast = uint8(TokenReuseDetected)
	enumName = "SecurityEvent"
)

type (
	// Enum - тип события журнала безопасности.
	Enum uint8
)

//nolint:gochecknoglobals,gosec // G101: имена событий, а не учётные данные
var (
	enumKeys = map[Enum]string{
		SignedIn:                 "SIGNED_IN",
		Auth2FAEnabled:           "AUTH_2FA_ENABLED",
		Auth2FADisabled:          "AUTH_2FA_DISABLED",
		RecoveryCodeUsed:         "RECOVERY_CODE_USED",
		RecoveryCodesRegenerated: "RECOVERY_CODES_REGENERATED",
		EmailChangeRequested:     "EMAIL_CHANGE_REQUESTED",
		EmailChanged:             "EMAIL_CHANGED",
		EmailChangeRevoked:       "EMAIL_CHANGE_REVOKED",
		PhoneChanged:             "PHONE_CHANGED",
		SessionsClosed:           "SESSIONS_CLOSED",
		TokenReuseDetected:       "TOKEN_REUSE_DETECTED",
	}
)

// Set - устанавливает указанное значение, если оно является enum значением.
func (e *Enum) Set(value uint8) error {
	if value > 0 && value <= enumLast {
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
