package dto

type (
	// UserSession - открытая сессия пользователя для выдачи в публичном API.
	// TODO: нужно добавить поле открытия сессии (created_at).
	UserSession struct {
		SessionID  uint32
		AppName    string
		DeviceName string
		LastIP     string
		Location   string
		IsCurrent  bool
	}
)
