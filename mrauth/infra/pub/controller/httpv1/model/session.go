package model

type (
	// CloseSessionsRequest - запрос на закрытие указанных сессий пользователя.
	CloseSessionsRequest struct {
		SessionIDs []string `json:"session_ids" validate:"required,gte=1,lte=64,dive,len=8,hexadecimal"`
	}

	// UserSessionResponse - открытая сессия пользователя.
	UserSessionResponse struct {
		SessionID  string `json:"session_id"`
		AppName    string `json:"app_name"`
		DeviceName string `json:"device_name"`
		LastIP     string `json:"last_ip"`
		Location   string `json:"location"`
		CreatedAt  string `json:"created_at"`
		LastSeenAt string `json:"last_seen_at"`
		IsCurrent  bool   `json:"is_current"`
	}
)
