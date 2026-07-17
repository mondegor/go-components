package model

type (
	// CloseSessionsRequest - запрос на закрытие указанных сессий пользователя.
	CloseSessionsRequest struct {
		SessionIDs []string `json:"session_ids" validate:"required,gte=1,lte=64,dive,len=8,hexadecimal"`
	}

	// UserSessionResponse - открытая сессия пользователя. Location отсутствует, если
	// местоположение не было вычислено; ExpiresAt - если срок жизни сессии не определён
	// (защита от нарушения инварианта "открытая сессия имеет действующий refresh токен").
	UserSessionResponse struct {
		SessionID  string `json:"session_id"`
		AppName    string `json:"app_name"`
		DeviceName string `json:"device_name"`
		LastIP     string `json:"last_ip"`
		Location   string `json:"location,omitempty"`
		CreatedAt  string `json:"created_at"`
		LastSeenAt string `json:"last_seen_at"`
		ExpiresAt  string `json:"expires_at,omitempty"`
		IsCurrent  bool   `json:"is_current"`
	}
)
