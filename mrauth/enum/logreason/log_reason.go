package logreason

// Причины провала/блокировки записи журнала защищённых операций.
// Нулевое значение UNSPECIFIED используется при успешных исходах.
const (
	Unspecified         Enum = iota // причина не указана (успех)
	WrongCode                       // неверный код подтверждения
	AttemptsExhausted               // исчерпаны попытки подтверждения
	Throttled                       // сработал троттлинг/анти-спам
	TokenReuse                      // повторное использование refresh-токена
	AccessForbidden                 // обращение к чужой операции
	TOTPReplay                      // повторное использование уже израсходованного второго фактора
	Expired                         // операция истекла (зарезервировано: сейчас не выставляется)
	NotConfirmed                    // операция не подтверждена
	LoginNotExists                  // логин не существует
	SessionLimit                    // превышен лимит сессий
	Superseded                      // операция вытеснена новой операцией того же типа
	ResendsExhausted                // исчерпаны повторные отправки кода подтверждения
	Auth2FAStateChanged             // состояние 2FA изменилось между созданием операции и её применением
	EmailChanged                    // емаил пользователя изменился после создания операции
)

type (
	// Enum - причина провала/блокировки записи журнала.
	Enum uint8
)

//nolint:gochecknoglobals
var (
	enumKeys = map[Enum]string{
		Unspecified:         "UNSPECIFIED",
		WrongCode:           "WRONG_CODE",
		AttemptsExhausted:   "ATTEMPTS_EXHAUSTED",
		Throttled:           "THROTTLED",
		TokenReuse:          "TOKEN_REUSE",
		AccessForbidden:     "ACCESS_FORBIDDEN",
		TOTPReplay:          "TOTP_REPLAY",
		Expired:             "EXPIRED",
		NotConfirmed:        "NOT_CONFIRMED",
		LoginNotExists:      "LOGIN_NOT_EXISTS",
		SessionLimit:        "SESSION_LIMIT",
		Superseded:          "SUPERSEDED",
		ResendsExhausted:    "RESENDS_EXHAUSTED",
		Auth2FAStateChanged: "AUTH_2FA_STATE_CHANGED",
		EmailChanged:        "EMAIL_CHANGED",
	}
)

// String - возвращает значение в виде строки.
func (e Enum) String() string {
	if v, ok := enumKeys[e]; ok {
		return v
	}

	return "UNKNOWN"
}
