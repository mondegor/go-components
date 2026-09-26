package model

type (
	// UserSessionsRequest - запрос списка открытых сессий пользователя.
	// Заполняется не из тела запроса, а из query-параметра ?realm= (json-тег задаёт имя
	// атрибута в ошибке валидации); realm необязателен: nil = realm текущей сессии.
	UserSessionsRequest struct {
		Realm *string `json:"realm,omitempty" validate:"omitnil,min=4,max=32,tag_realm"`
	}

	// CloseSessionsRequest - запрос на закрытие указанных сессий пользователя. Идентификатор -
	// ровно 8 шестнадцатеричных символов в нижнем регистре, как их отдаёт список сессий:
	// hexadecimal сам по себе пропускает префикс 0x/0X, его отсекают lowercase и excludes=x.
	CloseSessionsRequest struct {
		SessionIDs []string `json:"session_ids" validate:"required,gte=1,lte=64,dive,len=8,hexadecimal,lowercase,excludes=x"`
	}

	// UserSessionResponse - открытая сессия пользователя.
	UserSessionResponse struct {
		SessionID  string `json:"session_id"`
		AppName    string `json:"app_name"`
		DeviceName string `json:"device_name"`
		LastIP     string `json:"last_ip"`
		Location   string `json:"location,omitempty"`
		CreatedAt  string `json:"created_at"`
		LastSeenAt string `json:"last_seen_at"`
		ExpiresAt  string `json:"expires_at"`
		IsCurrent  bool   `json:"is_current"`
	}
)
