package logstatus

// Статусы записи журнала защищённых операций (крупный исход события).
const (
	Opened         Enum = iota + 1 // операция инициирована
	ResentCode                     // код подтверждения отправлен повторно
	ConfirmSuccess                 // действие подтверждено успешно
	ConfirmFailed                  // подтверждение действия не удалось
	Confirmed                      // операция полностью подтверждена
	Revoked                        // операция отозвана
	Applied                        // операция применена
	Blocked                        // событие заблокировано (атака/лимит/троттлинг)
	SessionOpened                  // сессия открыта, пользователь вошёл (токены выданы)
)

type (
	// Enum - статус записи журнала.
	Enum uint8
)

//nolint:gochecknoglobals
var (
	enumKeys = map[Enum]string{
		Opened:         "OPENED",
		ResentCode:     "RESENT_CODE",
		ConfirmSuccess: "CONFIRM_SUCCESS",
		ConfirmFailed:  "CONFIRM_FAILED",
		Confirmed:      "CONFIRMED",
		Revoked:        "REVOKED",
		Applied:        "APPLIED",
		Blocked:        "BLOCKED",
		SessionOpened:  "SESSION_OPENED",
	}
)

// String - возвращает значение в виде строки.
func (e Enum) String() string {
	if v, ok := enumKeys[e]; ok {
		return v
	}

	return "UNKNOWN"
}
